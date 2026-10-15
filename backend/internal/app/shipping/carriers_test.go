package shipping

import (
	"errors"
	"jstihuo/internal/app/identity"
	"testing"
)

// TestCarrierReferenceBoundary 用途：验证未接入 LIVE 时不能存入任意密钥引用。
// 参数：t 测试上下文。
// 返回值：无，路径、密钥内容和未配置引用不得通过后台输入。
func TestCarrierReferenceBoundary(t *testing.T) {
	for _, test := range []struct {
		provider, reference string
		status              int
	}{
		{"MANUAL", "", 0}, {"MOCK", "", 0}, {"KDNIAO", "", 0}, {"KUAIDI100", "", 0},
		{"KDNIAO", "unconfigured-provider", 503}, {"KUAIDI100", "../secret", 422}, {"MOCK", "not-allowed", 422},
	} {
		in := CarrierInput{Code: "TEST", Name: "测试承运商", Provider: test.provider, SecretRef: test.reference}
		err := validate(&in)
		if test.status == 0 {
			if err != nil {
				t.Fatal(err)
			}
			continue
		}
		var f *identity.Fault
		if !errors.As(err, &f) || f.Status != test.status {
			t.Fatal("引用边界不符", err)
		}
	}
}
