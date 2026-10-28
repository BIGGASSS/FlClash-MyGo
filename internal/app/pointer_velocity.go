package app

import (
	"math"
	"time"
)

type pointerSample struct {
	X  float64
	At time.Time
}

func pointerVelocity(samples []pointerSample, now time.Time) float64 {
	if len(samples) == 0 || now.Sub(samples[len(samples)-1].At) > 40*time.Millisecond {
		return 0
	}
	newest, previous := samples[len(samples)-1].At, samples[len(samples)-1].At
	var times, values []float64
	for i := len(samples) - 1; i >= 0 && len(times) < 20; i-- {
		sample := samples[i]
		age, gap := newest.Sub(sample.At), previous.Sub(sample.At)
		if age > 100*time.Millisecond || gap > 40*time.Millisecond {
			break
		}
		times = append(times, -float64(age)/float64(time.Millisecond))
		values = append(values, sample.X)
		previous = sample.At
	}
	if len(times) < 3 {
		return 0
	}
	var q [3][]float64
	var r [3][3]float64
	var projection [3]float64
	for degree := range 3 {
		column := make([]float64, len(times))
		for i, t := range times {
			column[i] = math.Pow(t, float64(degree))
		}
		for prior := 0; prior < degree; prior++ {
			for i, v := range column {
				r[prior][degree] += v * q[prior][i]
			}
			for i := range column {
				column[i] -= r[prior][degree] * q[prior][i]
			}
		}
		for _, v := range column {
			r[degree][degree] += v * v
		}
		r[degree][degree] = math.Sqrt(r[degree][degree])
		if r[degree][degree] < 1e-10 {
			return 0
		}
		for i := range column {
			column[i] /= r[degree][degree]
			projection[degree] += column[i] * values[i]
		}
		q[degree] = column
	}
	var coefficients [3]float64
	for degree := 2; degree >= 0; degree-- {
		v := projection[degree]
		for higher := degree + 1; higher < 3; higher++ {
			v -= r[degree][higher] * coefficients[higher]
		}
		coefficients[degree] = v / r[degree][degree]
	}
	return coefficients[1] * 1000
}
