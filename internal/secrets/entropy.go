package secrets

import "math"

func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]float64)
	for _, c := range s {
		freq[c]++
	}
	var entropy float64
	length := float64(len(s))
	for _, count := range freq {
		p := count / length
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	return entropy
}

const entropyThreshold = 4.5

func highEntropy(s string) bool {
	return len(s) >= 20 && shannonEntropy(s) >= entropyThreshold
}
