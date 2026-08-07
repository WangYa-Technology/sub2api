package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRedeemCodeExpiry(t *testing.T) {
	now := time.Now().UTC()
	past := now.Add(-time.Hour)
	future := now.Add(time.Hour)

	tests := []struct {
		name        string
		code        RedeemCode
		wantExpired bool
		wantCanUse  bool
	}{
		{
			name:        "unused without expiry can be used",
			code:        RedeemCode{Status: StatusUnused},
			wantExpired: false,
			wantCanUse:  true,
		},
		{
			name:        "unused before expiry can be used",
			code:        RedeemCode{Status: StatusUnused, ExpiresAt: &future},
			wantExpired: false,
			wantCanUse:  true,
		},
		{
			name:        "unused after expiry cannot be used",
			code:        RedeemCode{Status: StatusUnused, ExpiresAt: &past},
			wantExpired: true,
			wantCanUse:  false,
		},
		{
			name:        "explicit expired status is expired",
			code:        RedeemCode{Status: StatusExpired},
			wantExpired: true,
			wantCanUse:  false,
		},
		{
			name:        "used code remains used even after expiry time",
			code:        RedeemCode{Status: StatusUsed, ExpiresAt: &past},
			wantExpired: false,
			wantCanUse:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.wantExpired, tt.code.IsExpiredAt(now))
			require.Equal(t, tt.wantCanUse, tt.code.CanUse())
		})
	}
}

func TestRedeemCodeAffiliateRebateBaseAmount(t *testing.T) {
	// SalePrice（人民币销售价）按模型广场充值倍率换算为美元返利基数。
	require.InDelta(t, 10/6.8, redeemCodeAffiliateRebateBaseAmount(&RedeemCode{
		Type:      RedeemTypeSubscription,
		SalePrice: 10,
	}, 6.8), 0.0000001)
	require.InDelta(t, 100.0, redeemCodeAffiliateRebateBaseAmount(&RedeemCode{
		Type:      RedeemTypeSubscription,
		SalePrice: 680,
	}, 6.8), 0.0000001)
	// 余额兑换码保留旧行为：Value 直接作为返利基数。
	require.Equal(t, 25.0, redeemCodeAffiliateRebateBaseAmount(&RedeemCode{
		Type:  RedeemTypeBalance,
		Value: 25,
	}, 6.8))
	// 非销售价、非余额类型不触发返利。
	require.Zero(t, redeemCodeAffiliateRebateBaseAmount(&RedeemCode{
		Type:  RedeemTypeSubscription,
		Value: 30,
	}, 6.8))
	// 汇率无效（<=0）时不发放销售价返利，避免错误放大基数。
	require.Zero(t, redeemCodeAffiliateRebateBaseAmount(&RedeemCode{
		Type:      RedeemTypeSubscription,
		SalePrice: 10,
	}, 0))
	// nil 兑换码不触发返利。
	require.Zero(t, redeemCodeAffiliateRebateBaseAmount(nil, 6.8))
}
