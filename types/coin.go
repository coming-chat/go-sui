package types

import (
	"errors"
	"github.com/shopspring/decimal"
	"math/big"
	"sort"
)

// type LockedBalance struct {
// 	EpochId int64 `json:"epochId"`
// 	Number  int64 `json:"number"`
// }

type Coin struct {
	CoinType     string            `json:"coinType"`
	CoinObjectId ObjectId          `json:"coinObjectId"`
	Version      decimal.Decimal   `json:"version"`
	Digest       TransactionDigest `json:"digest"`
	Balance      decimal.Decimal   `json:"balance"`

	LockedUntilEpoch    *decimal.Decimal  `json:"lockedUntilEpoch,omitempty"`
	PreviousTransaction TransactionDigest `json:"previousTransaction"`
}

type CoinPage = Page[Coin, ObjectId]

type Balance struct {
	CoinType        string                              `json:"coinType"`
	CoinObjectCount uint64                              `json:"coinObjectCount"`
	TotalBalance    decimal.Decimal                     `json:"totalBalance"`
	LockedBalance   map[decimal.Decimal]decimal.Decimal `json:"lockedBalance"`
}

type Supply struct {
	Value decimal.Decimal `json:"value"`
}

var ErrCoinsNotMatchRequest error
var ErrCoinsNeedMoreObject error

const (
	PickSmaller = iota // pick smaller coins to match amount
	PickBigger         // pick bigger coins to match amount
	PickByOrder        // pick coins by coins order to match amount
)

// type Coin struct {
// 	Balance   uint64     `json:"balance"`
// 	Type      string     `json:"type"`
// 	Owner     *Address   `json:"owner"`
// 	Reference *ObjectRef `json:"reference"`
// }

func (c *Coin) Reference() *ObjectRef {
	return &ObjectRef{
		Digest:   c.Digest,
		Version:  c.Version.BigInt().Uint64(),
		ObjectId: c.CoinObjectId,
	}
}

type Coins []Coin

func init() {
	ErrCoinsNotMatchRequest = errors.New("coins not match request")
	ErrCoinsNeedMoreObject = errors.New("you should get more SUI coins and try again")
}

func (cs Coins) TotalBalance() *big.Int {
	total := big.NewInt(0)
	for _, coin := range cs {
		total = total.Add(total, coin.Balance.BigInt())
	}
	return total
}

func (cs Coins) PickCoinNoLess(amount uint64) (*Coin, error) {
	for i, coin := range cs {
		if coin.Balance.BigInt().Uint64() >= amount {
			cs = append(cs[:i], cs[i+1:]...)
			return &coin, nil
		}
	}
	if len(cs) <= 3 {
		return nil, errors.New("insufficient balance")
	}
	return nil, errors.New("no coin is enough to cover the gas")
}

// PickSUICoinsWithGas pick coins, which sum >= amount, and pick a gas coin >= gasAmount which not in coins
// if not satisfated amount/gasAmount, an ErrCoinsNotMatchRequest/ErrCoinsNeedMoreObject error will return
// if gasAmount == 0, a nil gasCoin will return
// pickMethod, see PickSmaller|PickBigger|PickByOrder
func (cs Coins) PickSUICoinsWithGas(amount *big.Int, gasAmount uint64, pickMethod int) (Coins, *Coin, error) {
	if gasAmount == 0 {
		res, err := cs.PickCoins(amount, pickMethod)
		return res, nil, err
	}

	if amount.Cmp(big.NewInt(0)) == 0 && gasAmount == 0 {
		return make(Coins, 0), nil, nil
	} else if len(cs) == 0 {
		return cs, nil, ErrCoinsNeedMoreObject
	}

	// find smallest to match gasAmount
	var gasCoin *Coin
	var selectIndex int
	for i := range cs {
		if cs[i].Balance.BigInt().Uint64() < gasAmount {
			continue
		}

		if nil == gasCoin || gasCoin.Balance.GreaterThan(cs[i].Balance) {
			gasCoin = &cs[i]
			selectIndex = i
		}
	}
	if nil == gasCoin {
		return nil, nil, ErrCoinsNotMatchRequest
	}

	lastCoins := make(Coins, 0, len(cs)-1)
	lastCoins = append(lastCoins, cs[0:selectIndex]...)
	lastCoins = append(lastCoins, cs[selectIndex+1:]...)
	pickCoins, err := lastCoins.PickCoins(amount, pickMethod)
	return pickCoins, gasCoin, err
}

// PickCoins pick coins, which sum >= amount,
// pickMethod, see PickSmaller|PickBigger|PickByOrder
// if not satisfated amount, an ErrCoinsNeedMoreObject error will return
func (cs Coins) PickCoins(amount *big.Int, pickMethod int) (Coins, error) {
	var sortedCoins Coins
	if pickMethod == PickByOrder {
		sortedCoins = cs
	} else {
		sortedCoins = make(Coins, len(cs))
		copy(sortedCoins, cs)
		sort.Slice(
			sortedCoins, func(i, j int) bool {
				if pickMethod == PickSmaller {
					return sortedCoins[i].Balance.LessThan(sortedCoins[j].Balance)
				} else {
					return sortedCoins[i].Balance.GreaterThanOrEqual(sortedCoins[j].Balance)
				}
			},
		)
	}

	result := make(Coins, 0)
	total := big.NewInt(0)
	for _, coin := range sortedCoins {
		result = append(result, coin)
		total = new(big.Int).Add(total, coin.Balance.BigInt())
		if total.Cmp(amount) >= 0 {
			return result, nil
		}
	}

	return nil, ErrCoinsNeedMoreObject
}
