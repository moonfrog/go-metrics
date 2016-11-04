package metrics

type Metric interface {
	Update(val int64)
}
