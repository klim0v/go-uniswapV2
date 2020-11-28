package uniswapV2

import (
	"errors"
	"math/big"
	"sync"
)

const minimumLiquidity int64 = 1000

type Token int32
type Address string

const addressZero Address = ""

type UniswapV2 struct {
	muPairs         sync.RWMutex
	pairs           map[pairKey]*Pair
	keyPairs        []pairKey
	isDirtyKeyPairs bool
}

func New() *UniswapV2 {
	return &UniswapV2{pairs: map[pairKey]*Pair{}}
}

var mainPrefix = "p"

type balance struct {
	address   Address
	liquidity *big.Int
}

type pairData struct {
	*sync.RWMutex
	reserve0    *big.Int
	reserve1    *big.Int
	totalSupply *big.Int
}

func (pd *pairData) TotalSupply() *big.Int {
	pd.RLock()
	defer pd.RUnlock()
	return pd.totalSupply
}

func (pd *pairData) Reserves() (reserve0 *big.Int, reserve1 *big.Int) {
	pd.RLock()
	defer pd.RUnlock()
	return pd.reserve0, pd.reserve1
}

func (pd *pairData) Revert() pairData {
	return pairData{
		RWMutex:     pd.RWMutex,
		reserve0:    pd.reserve1,
		reserve1:    pd.reserve0,
		totalSupply: pd.totalSupply,
	}
}

func (s *UniswapV2) Pairs() ([]pairKey, error) {
	s.muPairs.Lock()
	defer s.muPairs.Unlock()

	return s.keyPairs, nil
}

func (s *UniswapV2) pair(key pairKey) (*Pair, bool) {
	if key.isSorted() {
		pair, ok := s.pairs[key]
		return pair, ok
	}
	pair, ok := s.pairs[key.sort()]
	if !ok {
		return nil, false
	}
	return &Pair{
		muBalance: pair.muBalance,
		pairData:  pair.pairData.Revert(),
		balances:  pair.balances,
		dirty:     pair.dirty,
	}, true
}

func (s *UniswapV2) Pair(coinA, coinB Token) *Pair {
	s.muPairs.Lock()
	defer s.muPairs.Unlock()

	key := pairKey{TokenA: coinA, TokenB: coinB}
	pair, _ := s.pair(key)
	return pair
}

type pairKey struct {
	TokenA, TokenB Token
}

func (pk pairKey) sort() pairKey {
	if pk.isSorted() {
		return pk
	}
	return pk.Revert()
}

func (pk pairKey) isSorted() bool {
	return pk.TokenA < pk.TokenB
}

func (pk pairKey) Revert() pairKey {
	return pairKey{TokenA: pk.TokenB, TokenB: pk.TokenA}
}

var (
	ErrorIdenticalAddresses = errors.New("IDENTICAL_ADDRESSES")
	ErrorPairExists         = errors.New("PAIR_EXISTS")
)

func (s *UniswapV2) CreatePair(coinA, coinB Token) (*Pair, error) {
	if coinA == coinB {
		return nil, ErrorIdenticalAddresses
	}

	pair := s.Pair(coinA, coinB)
	if pair != nil {
		return nil, ErrorPairExists
	}

	totalSupply, reserve0, reserve1, balances := big.NewInt(0), big.NewInt(0), big.NewInt(0), map[Address]*big.Int{}

	s.muPairs.Lock()
	defer s.muPairs.Unlock()

	key := pairKey{coinA, coinB}
	pair = s.addPair(key, pairData{reserve0: reserve0, reserve1: reserve1, totalSupply: totalSupply}, balances)
	s.addKeyPair(key)
	if !key.isSorted() {
		return &Pair{
			muBalance: pair.muBalance,
			pairData:  pair.Revert(),
			balances:  pair.balances,
			dirty:     pair.dirty,
		}, nil
	}
	return pair, nil
}

func (s *UniswapV2) addPair(key pairKey, data pairData, balances map[Address]*big.Int) *Pair {
	if !key.isSorted() {
		key.Revert()
		data = data.Revert()
	}
	data.RWMutex = &sync.RWMutex{}
	pair := &Pair{
		muBalance: &sync.RWMutex{},
		pairData:  data,
		balances:  balances,
		dirty: &dirty{
			isDirty:         false,
			isDirtyBalances: false,
		},
	}
	s.pairs[key] = pair
	return pair
}

func (s *UniswapV2) addKeyPair(key pairKey) {
	s.keyPairs = append(s.keyPairs, key.sort())
	s.isDirtyKeyPairs = true
}

var (
	ErrorInsufficientLiquidityMinted = errors.New("INSUFFICIENT_LIQUIDITY_MINTED")
)

type dirty struct {
	isDirty         bool
	isDirtyBalances bool
}
type Pair struct {
	pairData
	muBalance *sync.RWMutex
	balances  map[Address]*big.Int
	*dirty
}

func (p *Pair) Balance(address Address) (liquidity *big.Int) {
	p.muBalance.RLock()
	defer p.muBalance.RUnlock()

	balance := p.balances[address]
	if balance == nil {
		return nil
	}

	return new(big.Int).Set(balance)
}

func (p *Pair) Mint(address Address, amount0, amount1 *big.Int) (liquidity *big.Int, err error) {
	if p.TotalSupply().Sign() == 0 {
		liquidity = startingSupply(amount0, amount1)
		if liquidity.Sign() != 1 {
			return nil, ErrorInsufficientLiquidityMinted
		}
		p.mint(addressZero, big.NewInt(minimumLiquidity))
	} else {
		liquidity := new(big.Int).Div(new(big.Int).Mul(p.totalSupply, amount0), p.reserve0)
		liquidity1 := new(big.Int).Div(new(big.Int).Mul(p.totalSupply, amount1), p.reserve1)
		if liquidity.Cmp(liquidity1) == 1 {
			liquidity = liquidity1
		}
	}

	p.mint(address, liquidity)
	p.update(amount0, amount1)

	return liquidity, nil
}

var (
	ErrorInsufficientLiquidityBurned = errors.New("INSUFFICIENT_LIQUIDITY_BURNED")
)

func (p *Pair) Burn(address Address, liquidity *big.Int) (amount0 *big.Int, amount1 *big.Int, err error) {
	balance := p.Balance(address)
	if balance == nil {
		return nil, nil, ErrorInsufficientLiquidityBurned
	}

	if liquidity.Cmp(balance) == 1 {
		return nil, nil, ErrorInsufficientLiquidityBurned
	}

	amount0, amount1 = p.Amounts(liquidity)

	if amount0.Sign() != 1 || amount1.Sign() != 1 {
		return nil, nil, ErrorInsufficientLiquidityBurned
	}

	p.burn(address, liquidity)
	p.update(new(big.Int).Neg(amount0), new(big.Int).Neg(amount1))

	return amount0, amount1, nil
}

var (
	ErrorK                        = errors.New("K")
	ErrorInsufficientInputAmount  = errors.New("INSUFFICIENT_INPUT_AMOUNT")
	ErrorInsufficientOutputAmount = errors.New("INSUFFICIENT_OUTPUT_AMOUNT")
	ErrorInsufficientLiquidity    = errors.New("INSUFFICIENT_LIQUIDITY")
)

func (p *Pair) Swap(amount0In, amount1In, amount0Out, amount1Out *big.Int) (amount0, amount1 *big.Int, err error) {
	if amount0Out.Sign() != 1 && amount1Out.Sign() != 1 {
		return nil, nil, ErrorInsufficientOutputAmount
	}

	reserve0, reserve1 := p.Reserves()

	if amount0Out.Cmp(reserve0) == 1 || amount1Out.Cmp(reserve1) == 1 {
		return nil, nil, ErrorInsufficientLiquidity
	}

	amount0 = new(big.Int).Sub(amount0In, amount0Out)
	amount1 = new(big.Int).Sub(amount1In, amount1Out)

	if amount0.Sign() != 1 && amount1.Sign() != 1 {
		return nil, nil, ErrorInsufficientInputAmount
	}

	balance0Adjusted := new(big.Int).Sub(new(big.Int).Mul(new(big.Int).Add(amount0, reserve0), big.NewInt(1000)), new(big.Int).Mul(amount0In, big.NewInt(3)))
	balance1Adjusted := new(big.Int).Sub(new(big.Int).Mul(new(big.Int).Add(amount1, reserve1), big.NewInt(1000)), new(big.Int).Mul(amount1In, big.NewInt(3)))

	if new(big.Int).Mul(balance0Adjusted, balance1Adjusted).Cmp(new(big.Int).Mul(new(big.Int).Mul(reserve0, reserve1), big.NewInt(1000000))) == -1 {
		return nil, nil, ErrorK
	}

	p.update(amount0, amount1)

	return amount0, amount1, nil
}

func (p *Pair) mint(address Address, value *big.Int) {
	p.pairData.Lock()
	defer p.pairData.Unlock()

	p.muBalance.Lock()
	defer p.muBalance.Unlock()

	p.isDirtyBalances = true
	p.isDirty = true
	p.totalSupply.Add(p.totalSupply, value)
	balance := p.balances[address]
	if balance == nil {
		p.balances[address] = big.NewInt(0)
	}
	p.balances[address].Add(p.balances[address], value)
}

func (p *Pair) burn(address Address, value *big.Int) {
	p.pairData.Lock()
	defer p.pairData.Unlock()
	p.muBalance.Lock()
	defer p.muBalance.Unlock()

	p.isDirtyBalances = true
	p.isDirty = true
	p.balances[address].Sub(p.balances[address], value)
	p.totalSupply.Sub(p.totalSupply, value)
}

func (p *Pair) update(amount0, amount1 *big.Int) {
	p.pairData.Lock()
	defer p.pairData.Unlock()

	p.isDirty = true
	p.reserve0.Add(p.reserve0, amount0)
	p.reserve1.Add(p.reserve1, amount1)
}

func (p *Pair) Amounts(liquidity *big.Int) (amount0 *big.Int, amount1 *big.Int) {
	p.pairData.RLock()
	defer p.pairData.RUnlock()
	amount0 = new(big.Int).Div(new(big.Int).Mul(liquidity, p.reserve0), p.totalSupply)
	amount1 = new(big.Int).Div(new(big.Int).Mul(liquidity, p.reserve1), p.totalSupply)
	return amount0, amount1
}

func startingSupply(amount0 *big.Int, amount1 *big.Int) *big.Int {
	mul := new(big.Int).Mul(amount0, amount1)
	sqrt := new(big.Int).Sqrt(mul)
	return new(big.Int).Sub(sqrt, big.NewInt(minimumLiquidity))
}
