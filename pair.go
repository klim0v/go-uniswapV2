package uniswapV2

import (
	"errors"
	"math/big"
	"sync"
)

const minimumLiquidity int64 = 1000
const addressZero Address = "addressZero"

type pairKey struct {
	tokenA, tokenB Token
}
type Service struct {
	muPairs sync.RWMutex
	pairs   map[pairKey]*Pair
	Pairs   []*Pair
}

func (s *Service) allPairsLength() int {
	s.muPairs.RLock()
	defer s.muPairs.RUnlock()
	return len(s.Pairs)
}

func (s *Service) Pair(tokenA, tokenB Token) *Pair {
	s.muPairs.RLock()
	defer s.muPairs.RUnlock()
	return s.pairs[pairKey{tokenA: tokenA, tokenB: tokenB}]
}

type Token uint32
type Address string

func New() *Service {
	return &Service{pairs: map[pairKey]*Pair{}}
}

type Pair struct {
	mu sync.RWMutex
	pairKey
	reserve0, reserve1 *big.Int
	TotalSupply        *big.Int
	balances           map[Address]*big.Int
}

var (
	IdenticalAddressesError = errors.New("IDENTICAL_ADDRESSES")
	PairExistsError         = errors.New("PAIR_EXISTS")
)

func (s *Service) CreatePair(tokenA, tokenB Token) (*Pair, error) {
	if tokenA == tokenB {
		return nil, IdenticalAddressesError
	}

	if s.Pair(tokenA, tokenB) != nil {
		return nil, PairExistsError
	}

	totalSupply, reserve0, reserve1, balances := big.NewInt(0), big.NewInt(0), big.NewInt(0), map[Address]*big.Int{}

	s.muPairs.Lock()
	defer s.muPairs.Unlock()


	key := pairKey{
		tokenA: tokenA,
		tokenB: tokenB,
	}
	pair := &Pair{
		pairKey:     key,
		reserve0:    reserve0,
		reserve1:    reserve1,
		TotalSupply: totalSupply,
		balances:    balances,
	}
	s.pairs[key] = pair

	reverseKey := pairKey{
		tokenA: tokenB,
		tokenB: tokenA,
	}
	s.pairs[reverseKey] = &Pair{
		pairKey:     reverseKey,
		reserve0:    reserve1,
		reserve1:    reserve0,
		TotalSupply: totalSupply,
		balances:    balances,
	}

	s.Pairs = append(s.Pairs, pair)

	return pair, nil
}

var (
	InsufficientLiquidityMintedError = errors.New("INSUFFICIENT_LIQUIDITY_MINTED")
)

func (p *Pair) Balance(address Address) (liquidity *big.Int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.balances[address]
}
func (p *Pair) Mint(address Address, amount0, amount1 *big.Int) (liquidity *big.Int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.TotalSupply.Sign() == 0 {
		liquidity = startingSupply(amount0, amount1)
		if liquidity.Sign() != 1 {
			return nil, InsufficientLiquidityMintedError
		}
		p.mint(addressZero, big.NewInt(minimumLiquidity))
	} else {
		liquidity := new(big.Int).Div(new(big.Int).Mul(p.TotalSupply, amount0), p.reserve0)
		liquidity1 := new(big.Int).Div(new(big.Int).Mul(p.TotalSupply, amount1), p.reserve1)
		if liquidity.Cmp(liquidity1) == 1 {
			liquidity = liquidity1
		}
	}

	p.mint(address, liquidity)
	p.update(amount0, amount1)
	// todo: kLast: if (feeOn) kLast = uint(reserve0).mul(reserve1); // reserve0 and reserve1 are up-to-date
	return liquidity, nil
}

var (
	InsufficientLiquidityBurnedError = errors.New("INSUFFICIENT_LIQUIDITY_BURNED")
)

func (p *Pair) Burn(address Address, liquidity *big.Int) (amount0 *big.Int, amount1 *big.Int, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	balance := p.balances[address]
	if balance == nil {
		return nil, nil, InsufficientLiquidityBurnedError
	}

	if liquidity.Cmp(balance) == 1 {
		return nil, nil, InsufficientLiquidityBurnedError
	}

	amount0, amount1 = p.amounts(liquidity)

	if amount0.Sign() != 1 || amount1.Sign() != 1 {
		return nil, nil, InsufficientLiquidityBurnedError
	}

	p.burn(address, liquidity)
	p.update(new(big.Int).Neg(amount0), new(big.Int).Neg(amount1))

	return amount0, amount1, nil
}

var (
	KError                        = errors.New("K")
	InsufficientInputAmountError  = errors.New("INSUFFICIENT_INPUT_AMOUNT")
	InsufficientOutputAmountError = errors.New("INSUFFICIENT_OUTPUT_AMOUNT")
	InsufficientLiquidityError    = errors.New("INSUFFICIENT_LIQUIDITY")
)

func (p *Pair) Swap(amount0In, amount1In, amount0Out, amount1Out *big.Int) (amount0, amount1 *big.Int, err error) {
	if amount0Out.Sign() != 1 && amount1Out.Sign() != 1 {
		return nil, nil, InsufficientOutputAmountError
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	if amount0Out.Cmp(p.reserve0) == 1 || amount1Out.Cmp(p.reserve1) == 1 {
		return nil, nil, InsufficientLiquidityError
	}

	amount0 = new(big.Int).Sub(amount0In, amount0Out)
	amount1 = new(big.Int).Sub(amount1In, amount1Out)

	if amount0.Sign() != 1 && amount1.Sign() != 1 {
		return nil, nil, InsufficientInputAmountError
	}

	balance0Adjusted := new(big.Int).Sub(new(big.Int).Mul(new(big.Int).Add(amount0, p.reserve0), big.NewInt(1000)), new(big.Int).Mul(amount0In, big.NewInt(3)))
	balance1Adjusted := new(big.Int).Sub(new(big.Int).Mul(new(big.Int).Add(amount1, p.reserve1), big.NewInt(1000)), new(big.Int).Mul(amount1In, big.NewInt(3)))

	if new(big.Int).Mul(balance0Adjusted, balance1Adjusted).Cmp(new(big.Int).Mul(new(big.Int).Mul(p.reserve0, p.reserve1), big.NewInt(1000000))) == -1 {
		return nil, nil, KError
	}

	p.update(amount0, amount1)

	return amount0, amount1, nil
}

func (p *Pair) mint(address Address, value *big.Int) {
	p.TotalSupply.Add(p.TotalSupply, value)
	balance := p.balances[address]
	if balance == nil {
		p.balances[address] = big.NewInt(0)
	}
	p.balances[address].Add(p.balances[address], value)
}

func (p *Pair) burn(address Address, value *big.Int) {
	p.balances[address].Sub(p.balances[address], value)
	p.TotalSupply.Sub(p.TotalSupply, value)
}

func (p *Pair) update(amount0, amount1 *big.Int) {
	p.reserve0.Add(p.reserve0, amount0)
	p.reserve1.Add(p.reserve1, amount1)
}

func (p *Pair) Amounts(liquidity *big.Int) (amount0 *big.Int, amount1 *big.Int) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.amounts(liquidity)
}

func (p *Pair) amounts(liquidity *big.Int) (amount0 *big.Int, amount1 *big.Int) {
	amount0 = new(big.Int).Div(new(big.Int).Mul(liquidity, p.reserve0), p.TotalSupply)
	amount1 = new(big.Int).Div(new(big.Int).Mul(liquidity, p.reserve1), p.TotalSupply)
	return amount0, amount1
}

func startingSupply(amount0 *big.Int, amount1 *big.Int) *big.Int {
	mul := new(big.Int).Mul(amount0, amount1)
	sqrt := new(big.Int).Sqrt(mul)
	return new(big.Int).Sub(sqrt, big.NewInt(minimumLiquidity))
}
