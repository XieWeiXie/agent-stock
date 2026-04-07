package indicator

import (
	"math"
	"testing"
)

func TestEMA(t *testing.T) {
	v := []float64{1, 2, 3, 4, 5}
	out := EMA(v, 3)
	if len(out) != len(v) {
		t.Fatalf("len mismatch: got %d want %d", len(out), len(v))
	}
	if out[0] != 1 {
		t.Fatalf("ema[0]=%v want 1", out[0])
	}
	if out[len(out)-1] <= 0 {
		t.Fatalf("ema last should be positive")
	}
}

func TestBOLL_Constant(t *testing.T) {
	close := make([]float64, 30)
	for i := range close {
		close[i] = 10
	}
	out := BOLL(close, 20, 2)
	last := out[len(out)-1]
	if last.Mid != 10 {
		t.Fatalf("mid=%v want 10", last.Mid)
	}
	if last.Upper != 10 || last.Lower != 10 {
		t.Fatalf("upper/lower=%v/%v want 10/10", last.Upper, last.Lower)
	}
}

func TestKDJ_Constant(t *testing.T) {
	high := make([]float64, 30)
	low := make([]float64, 30)
	close := make([]float64, 30)
	for i := 0; i < 30; i++ {
		high[i] = 10
		low[i] = 10
		close[i] = 10
	}
	out := KDJ(high, low, close, 9)
	last := out[len(out)-1]
	if math.Abs(last.K-50) > 1e-9 || math.Abs(last.D-50) > 1e-9 || math.Abs(last.J-50) > 1e-9 {
		t.Fatalf("kdj=%v/%v/%v want 50/50/50", last.K, last.D, last.J)
	}
}

func TestRSI_Increasing(t *testing.T) {
	close := make([]float64, 50)
	for i := range close {
		close[i] = float64(i + 1)
	}
	out := RSI(close, 14)
	last := out[len(out)-1]
	if last < 70 {
		t.Fatalf("rsi last=%v want >=70 for strictly increasing series", last)
	}
}

