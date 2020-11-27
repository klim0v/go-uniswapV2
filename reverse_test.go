package uniswapV2_test

import (
	"math/big"
	"testing"

	"github.com/klim0v/uniswapV2"
)

func TestReverseToken(t *testing.T) {
	service := uniswapV2.New()
	address := uniswapV2.Address("address")

	{
		pair, err := service.CreatePair(0, 1)
		if err != nil {
			t.Fatal(err)
		}

		token0Amount := new(big.Int).Add(new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18)), big.NewInt(0))
		token1Amount := new(big.Int).Add(new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18)), big.NewInt(0))
		_, err = pair.Mint(address, token0Amount, token1Amount)
		if err != nil {
			t.Fatal(err)
		}
	}
	{
		p01 := service.Pair(0, 1)
		p01AddressLiquidity := p01.Balance(address)
		p01amount0, p01amount1 := p01.Amounts(p01AddressLiquidity)

		p10 := service.Pair(1, 0)
		p10AddressLiquidity := p10.Balance(address)
		p10amount0, p10amount1 := p10.Amounts(p10AddressLiquidity)

		if p10AddressLiquidity.Cmp(p01AddressLiquidity) != 0 {
			t.Error("Balances not equal")
		}

		if p10amount0.Cmp(p01amount1) != 0 {
			t.Error("amount1 of pear {0,1} and amount0 of pear {1,0} a not equal")
		}

		if p10amount1.Cmp(p01amount0) != 0 {
			t.Error("amount0 of pear {0,1} and amount1 of pear {1,0} a not equal")
		}
	}
}
