package indicator

import (
	"math"
)

func EMA(values []float64, period int) []float64 {
	out := make([]float64, len(values))
	if len(values) == 0 {
		return out
	}
	if period <= 1 {
		copy(out, values)
		return out
	}
	k := 2.0 / (float64(period) + 1.0)
	out[0] = values[0]
	for i := 1; i < len(values); i++ {
		out[i] = values[i]*k + out[i-1]*(1.0-k)
	}
	return out
}

type BOLLPoint struct {
	Mid   float64
	Upper float64
	Lower float64
}

func BOLL(close []float64, period int, k float64) []BOLLPoint {
	out := make([]BOLLPoint, len(close))
	if period <= 1 || len(close) == 0 {
		return out
	}
	for i := range close {
		if i+1 < period {
			continue
		}
		start := i + 1 - period
		mean := 0.0
		for j := start; j <= i; j++ {
			mean += close[j]
		}
		mean /= float64(period)
		variance := 0.0
		for j := start; j <= i; j++ {
			d := close[j] - mean
			variance += d * d
		}
		variance /= float64(period)
		std := math.Sqrt(variance)
		out[i] = BOLLPoint{
			Mid:   mean,
			Upper: mean + k*std,
			Lower: mean - k*std,
		}
	}
	return out
}

type KDJPoint struct {
	K float64
	D float64
	J float64
}

func KDJ(high []float64, low []float64, close []float64, period int) []KDJPoint {
	out := make([]KDJPoint, len(close))
	if len(close) == 0 || len(high) != len(close) || len(low) != len(close) {
		return out
	}
	if period <= 1 {
		for i := range close {
			out[i] = KDJPoint{K: 50, D: 50, J: 50}
		}
		return out
	}
	kPrev, dPrev := 50.0, 50.0
	for i := range close {
		if i+1 < period {
			out[i] = KDJPoint{K: kPrev, D: dPrev, J: 3*kPrev - 2*dPrev}
			continue
		}
		start := i + 1 - period
		highest := high[start]
		lowest := low[start]
		for j := start + 1; j <= i; j++ {
			if high[j] > highest {
				highest = high[j]
			}
			if low[j] < lowest {
				lowest = low[j]
			}
		}
		rsv := 50.0
		if highest != lowest {
			rsv = (close[i] - lowest) / (highest - lowest) * 100
		}
		kCur := (2.0/3.0)*kPrev + (1.0/3.0)*rsv
		dCur := (2.0/3.0)*dPrev + (1.0/3.0)*kCur
		out[i] = KDJPoint{K: kCur, D: dCur, J: 3*kCur - 2*dCur}
		kPrev, dPrev = kCur, dCur
	}
	return out
}

func RSI(close []float64, period int) []float64 {
	out := make([]float64, len(close))
	if len(close) == 0 || period <= 0 {
		return out
	}
	if len(close) == 1 {
		out[0] = 50
		return out
	}
	gain, loss := 0.0, 0.0
	for i := 1; i < len(close) && i <= period; i++ {
		d := close[i] - close[i-1]
		if d > 0 {
			gain += d
		} else {
			loss -= d
		}
	}
	avgGain := gain / float64(period)
	avgLoss := loss / float64(period)
	for i := 0; i < len(close); i++ {
		if i < period {
			out[i] = 0
			continue
		}
		if i == period {
			out[i] = rsiFromAvg(avgGain, avgLoss)
			continue
		}
		d := close[i] - close[i-1]
		g := 0.0
		l := 0.0
		if d > 0 {
			g = d
		} else {
			l = -d
		}
		avgGain = (avgGain*float64(period-1) + g) / float64(period)
		avgLoss = (avgLoss*float64(period-1) + l) / float64(period)
		out[i] = rsiFromAvg(avgGain, avgLoss)
	}
	return out
}

func rsiFromAvg(avgGain, avgLoss float64) float64 {
	if avgLoss == 0 {
		if avgGain == 0 {
			return 50
		}
		return 100
	}
	rs := avgGain / avgLoss
	return 100 - (100 / (1 + rs))
}
