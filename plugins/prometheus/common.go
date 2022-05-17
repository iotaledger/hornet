package prometheus

var (

	/*
		MWM: 10, Length: 8
		MWM: 11, Length: 15
		MWM: 12, Length: 45
		MWM: 13, Length: 133
		MWM: 14, Length: 399
		MWM: 15, Length: 1196
		MWM: 16, Length: 3588
		MWM: 17, Length: 10762
		MWM: 18, Length: 32286
	*/
	powBlockSizeBuckets = []float64{
		45,
		133,
		399,
		1196,
		3588,
		10762,
		32286,
	}
	powDurationBuckets = []float64{.1, .2, .5, 1, 2, 5, 10, 20, 50, 100, 200, 500}
)
