package uniswapV2

import (
	"fmt"
	"math/big"
	"testing"
)

func TestPair_feeToOff(t *testing.T) {
	tableTests := []struct {
		token0, token1                   Token
		token0Amount, token1Amount       *big.Int
		swapAmount, expectedOutputAmount *big.Int
		expectedLiquidity                *big.Int
	}{
		{
			token0:               0,
			token1:               1,
			token0Amount:         new(big.Int).Add(new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18)), big.NewInt(0)),
			token1Amount:         new(big.Int).Add(new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18)), big.NewInt(0)),
			swapAmount:           new(big.Int).Add(new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)), big.NewInt(0)),
			expectedOutputAmount: big.NewInt(996006981039903216),
			expectedLiquidity:    new(big.Int).Add(new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18)), big.NewInt(0)),
		},
	}
	service := New()
	for i, tt := range tableTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			pair, err := service.CreatePair(tt.token0, tt.token1)
			if err != nil {
				t.Fatal(err)
			}
			liquidity, err := pair.Mint("address", tt.token0Amount, tt.token1Amount)
			if err != nil {
				t.Fatal(err)
			}
			expectedLiquidity := new(big.Int).Sub(tt.expectedLiquidity, big.NewInt(minimumLiquidity))
			if liquidity.Cmp(expectedLiquidity) != 0 {
				t.Errorf("liquidity want %s, got %s", expectedLiquidity, liquidity)
			}

			_, _, err = pair.Swap(big.NewInt(0), tt.swapAmount, tt.expectedOutputAmount, big.NewInt(0))
			if err != nil {
				t.Fatal(err)
			}

			_, _, err = pair.Burn("address", expectedLiquidity)
			if err != nil {
				t.Fatal(err)
			}

			if pair.TotalSupply.Cmp(big.NewInt(minimumLiquidity)) != 0 {
				t.Errorf("liquidity want %s, got %s", big.NewInt(minimumLiquidity), pair.TotalSupply)
			}
		})
	}
}

func TestPair_Mint(t *testing.T) {
	tableTests := []struct {
		token0, token1             Token
		token0Amount, token1Amount *big.Int
		expectedLiquidity          *big.Int
	}{
		{
			token0:            0,
			token1:            1,
			token0Amount:      new(big.Int).Add(new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)), big.NewInt(0)),
			token1Amount:      new(big.Int).Add(new(big.Int).Mul(big.NewInt(4), big.NewInt(1e18)), big.NewInt(0)),
			expectedLiquidity: new(big.Int).Add(new(big.Int).Mul(big.NewInt(2), big.NewInt(1e18)), big.NewInt(0)),
		},
	}
	service := New()
	for i, tt := range tableTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			pair, err := service.CreatePair(tt.token0, tt.token1)
			if err != nil {
				t.Fatal(err)
			}

			liquidity, err := pair.Mint("address", tt.token0Amount, tt.token1Amount)
			if err != nil {
				t.Fatal(err)
			}

			liquidityExpected := new(big.Int).Sub(tt.expectedLiquidity, big.NewInt(minimumLiquidity))
			if liquidity.Cmp(liquidityExpected) != 0 {
				t.Errorf("liquidity want %s, got %s", liquidityExpected, liquidity)
			}

			reserve0, reserve1 := pair.reserve0, pair.reserve1

			if reserve0.Cmp(tt.token0Amount) != 0 {
				t.Errorf("reserve0 want %s, got %s", tt.token0Amount, reserve0)
			}

			if reserve1.Cmp(tt.token1Amount) != 0 {
				t.Errorf("reserve1 want %s, got %s", tt.token1Amount, reserve1)
			}

			if pair.balances[addressZero].Cmp(big.NewInt(minimumLiquidity)) != 0 {
				t.Errorf("addressZero liquidity want %s, got %s", big.NewInt(minimumLiquidity), pair.balances[addressZero])
			}

			if pair.TotalSupply.Cmp(tt.expectedLiquidity) != 0 {
				t.Errorf("total supply want %s, got %s", big.NewInt(minimumLiquidity), pair.TotalSupply)
			}
		})
	}
}

func TestPair_Swap_token0(t *testing.T) {
	tableTests := []struct {
		token0, token1             Token
		token0Amount, token1Amount *big.Int
		swap0Amount                *big.Int
		swap1Amount                *big.Int
		expected0OutputAmount      *big.Int
		expected1OutputAmount      *big.Int
	}{
		{
			token0:                1,
			token1:                2,
			token0Amount:          new(big.Int).Add(new(big.Int).Mul(big.NewInt(5), big.NewInt(1e18)), big.NewInt(0)),
			token1Amount:          new(big.Int).Add(new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)), big.NewInt(0)),
			swap0Amount:           new(big.Int).Add(new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)), big.NewInt(0)),
			swap1Amount:           big.NewInt(0),
			expected0OutputAmount: big.NewInt(0),
			expected1OutputAmount: big.NewInt(1662497915624478906),
		},
	}
	service := New()
	for i, tt := range tableTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			pair, err := service.CreatePair(tt.token0, tt.token1)
			if err != nil {
				t.Fatal(err)
			}

			_, err = pair.Mint("address", tt.token0Amount, tt.token1Amount)
			if err != nil {
				t.Fatal(err)
			}

			_, _, err = pair.Swap(tt.swap0Amount, tt.swap1Amount, tt.expected0OutputAmount, new(big.Int).Add(tt.expected1OutputAmount, big.NewInt(1)))
			if err != KError {
				t.Fatalf("failed with %v; want error %v", err, KError)
			}

			amount0, amount1, err := pair.Swap(tt.swap0Amount, tt.swap1Amount, tt.expected0OutputAmount, tt.expected1OutputAmount)
			if err != nil {
				t.Fatal(err)
			}

			expected0Amount := new(big.Int).Add(tt.swap0Amount, tt.expected0OutputAmount)
			if amount0.Cmp(expected0Amount) != 0 {
				t.Errorf("amount0 want %s, got %s", expected0Amount, amount0)
			}

			expected1Amount := new(big.Int).Sub(tt.swap1Amount, tt.expected1OutputAmount)
			if amount1.Cmp(expected1Amount) != 0 {
				t.Errorf("amount1 want %s, got %s", expected1Amount, amount1)
			}

			if pair.reserve0.Cmp(new(big.Int).Add(tt.token0Amount, expected0Amount)) != 0 {
				t.Errorf("reserve0 want %s, got %s", new(big.Int).Add(tt.token0Amount, expected0Amount), pair.reserve0)
			}

			if pair.reserve1.Cmp(new(big.Int).Add(tt.token1Amount, expected1Amount)) != 0 {
				t.Errorf("reserve1 want %s, got %s", new(big.Int).Add(tt.token1Amount, expected1Amount), pair.reserve1)
			}
		})
	}
}

func TestPair_Swap_token1(t *testing.T) {
	tableTests := []struct {
		token0, token1             Token
		token0Amount, token1Amount *big.Int
		swap0Amount                *big.Int
		swap1Amount                *big.Int
		expected0OutputAmount      *big.Int
		expected1OutputAmount      *big.Int
	}{
		{
			token0:                1,
			token1:                2,
			token0Amount:          new(big.Int).Add(new(big.Int).Mul(big.NewInt(5), big.NewInt(1e18)), big.NewInt(0)),
			token1Amount:          new(big.Int).Add(new(big.Int).Mul(big.NewInt(10), big.NewInt(1e18)), big.NewInt(0)),
			swap0Amount:           big.NewInt(0),
			swap1Amount:           new(big.Int).Add(new(big.Int).Mul(big.NewInt(1), big.NewInt(1e18)), big.NewInt(0)),
			expected0OutputAmount: big.NewInt(453305446940074565),
			expected1OutputAmount: big.NewInt(0),
		},
	}
	service := New()
	for i, tt := range tableTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			pair, err := service.CreatePair(tt.token0, tt.token1)
			if err != nil {
				t.Fatal(err)
			}

			_, err = pair.Mint("address", tt.token0Amount, tt.token1Amount)
			if err != nil {
				t.Fatal(err)
			}

			_, _, err = pair.Swap(tt.swap0Amount, tt.swap1Amount, new(big.Int).Add(tt.expected0OutputAmount, big.NewInt(1)), tt.expected1OutputAmount)
			if err != KError {
				t.Fatalf("failed with %v; want error %v", err, KError)
			}
			amount0, amount1, err := pair.Swap(tt.swap0Amount, tt.swap1Amount, tt.expected0OutputAmount, tt.expected1OutputAmount)
			if err != nil {
				t.Fatal(err)
			}

			expected0Amount := new(big.Int).Sub(tt.swap0Amount, tt.expected0OutputAmount)
			if amount0.Cmp(expected0Amount) != 0 {
				t.Errorf("amount0 want %s, got %s", expected0Amount, amount0)
			}

			expected1Amount := new(big.Int).Sub(tt.swap1Amount, tt.expected1OutputAmount)
			if amount1.Cmp(expected1Amount) != 0 {
				t.Errorf("amount1 want %s, got %s", expected1Amount, amount1)
			}

			if pair.reserve0.Cmp(new(big.Int).Add(tt.token0Amount, expected0Amount)) != 0 {
				t.Errorf("reserve0 want %s, got %s", new(big.Int).Add(tt.token0Amount, expected0Amount), pair.reserve0)
			}

			if pair.reserve1.Cmp(new(big.Int).Add(tt.token1Amount, expected1Amount)) != 0 {
				t.Errorf("reserve1 want %s, got %s", new(big.Int).Add(tt.token1Amount, expected1Amount), pair.reserve1)
			}
		})
	}
}

func TestPair_Burn(t *testing.T) {
	tableTests := []struct {
		token0, token1             Token
		token0Amount, token1Amount *big.Int
		expectedLiquidity          *big.Int
	}{
		{
			token0:            0,
			token1:            1,
			token0Amount:      new(big.Int).Add(new(big.Int).Mul(big.NewInt(3), big.NewInt(1e18)), big.NewInt(0)),
			token1Amount:      new(big.Int).Add(new(big.Int).Mul(big.NewInt(3), big.NewInt(1e18)), big.NewInt(0)),
			expectedLiquidity: new(big.Int).Add(new(big.Int).Mul(big.NewInt(3), big.NewInt(1e18)), big.NewInt(0)),
		},
	}
	service := New()
	for i, tt := range tableTests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			pair, err := service.CreatePair(tt.token0, tt.token1)
			if err != nil {
				t.Fatal(err)
			}

			liquidity, err := pair.Mint("address", tt.token0Amount, tt.token1Amount)
			if err != nil {
				t.Fatal(err)
			}

			liquidityExpected := new(big.Int).Sub(tt.expectedLiquidity, big.NewInt(minimumLiquidity))
			if liquidity.Cmp(liquidityExpected) != 0 {
				t.Errorf("liquidity want %s, got %s", liquidityExpected, liquidity)
			}

			amount0, amount1, err := pair.Burn("address", liquidity)
			if err != nil {
				t.Fatal(err)
			}

			expectedAmount0 := new(big.Int).Sub(tt.token0Amount, big.NewInt(minimumLiquidity))
			if amount0.Cmp(expectedAmount0) != 0 {
				t.Errorf("amount0 want %s, got %s", expectedAmount0, amount0)
			}

			expectedAmount1 := new(big.Int).Sub(tt.token1Amount, big.NewInt(minimumLiquidity))
			if amount1.Cmp(expectedAmount1) != 0 {
				t.Errorf("amount1 want %s, got %s", expectedAmount1, amount1)
			}

			if pair.balances["address"].Sign() != 0 {
				t.Errorf("address liquidity want %s, got %s", "0", pair.balances["address"])
			}

			if pair.balances[addressZero].Cmp(big.NewInt(minimumLiquidity)) != 0 {
				t.Errorf("addressZero liquidity want %s, got %s", big.NewInt(minimumLiquidity), pair.balances[addressZero])
			}

			if pair.TotalSupply.Cmp(big.NewInt(minimumLiquidity)) != 0 {
				t.Errorf("total supply want %s, got %s", big.NewInt(minimumLiquidity), pair.TotalSupply)
			}
		})
	}
}
