package model

type JudgeStatus string

const (
	StatusAccepted            JudgeStatus = "AC"  // 答案正确
	StatusWrongAnswer         JudgeStatus = "WA"  // 答案错误
	StatusTimeLimitExceeded   JudgeStatus = "TLE" // 运行超时
	StatusMemoryLimitExceeded JudgeStatus = "MLE" // 内存超限
	StatusRuntimeError        JudgeStatus = "RE"  // 运行错误
	StatusSystemError         JudgeStatus = "SE"  // 系统内部错误
	StatusCompileError        JudgeStatus = "CE"  // 👈 新增：编译错误
)

type JudgeResult struct {
	Status       JudgeStatus
	Output       string
	Error        error
	TimeCost     int
	WallTimeCost int
	MemoryCost   int
	ExitCode     int
}
