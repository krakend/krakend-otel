package config

import (
	"go.opentelemetry.io/otel/metric"
)

var (
	TimeBucketsOpt = metric.WithExplicitBucketBoundaries(
		0.010, 0.020, 0.050, 0.075,
		0.100, 0.125, 0.150, 0.175,
		0.200, 0.250, 0.300, 0.350,
		0.500, 0.750, 1.000, 1.500,
		2.000, 3.500, 5.000, 10.000)

	SizeBucketsOpt = metric.WithExplicitBucketBoundaries(
		128, 256, 512, 1024, // <- reasonable response sizes
		4*1024, 8*1024, 16*1024, 32*1024, // <- these starts to be big
		64*1024, 4*64*1024, 8*64*1024, 16*64*1024, // <- those are huge for an api response 64k to 1 Meg
		4*1024*1024, 16*1024*1024, 64*1024*1024, // <- what are you sending here !? 64 Megs max detail ?
	)
)
