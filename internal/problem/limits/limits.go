package limits

const (
	DefaultTimeMS   = 1000
	MinTimeMS       = 100
	MaxTimeMS       = 10000
	DefaultMemoryMB = 256
	MinMemoryMB     = 16
	MaxMemoryMB     = 512
)

func ValidTimeMS(value int) bool {
	return value >= MinTimeMS && value <= MaxTimeMS
}

func ValidMemoryMB(value int) bool {
	return value >= MinMemoryMB && value <= MaxMemoryMB
}
