package model

type JudgeStatus string

const (
	StatusAccepted            JudgeStatus = "AC"  // 答案正确
	StatusWrongAnswer         JudgeStatus = "WA"  // 答案错误
	StatusTimeLimitExceeded   JudgeStatus = "TLE" // 运行超时
	StatusMemoryLimitExceeded JudgeStatus = "MLE" // 内存超限
	StatusOutputLimitExceeded JudgeStatus = "OLE" // 输出超限
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

// PublicMessage contains only verdict information safe to expose for formal judging.
func PublicMessage(status JudgeStatus) string {
	switch status {
	case StatusAccepted:
		return "所有测试用例均已通过。"
	case StatusWrongAnswer:
		return "输出与预期不符。"
	case StatusTimeLimitExceeded:
		return "程序运行超时。"
	case StatusMemoryLimitExceeded:
		return "程序运行内存超限。"
	case StatusOutputLimitExceeded:
		return "程序输出超限。"
	case StatusRuntimeError:
		return "程序运行异常。"
	case StatusCompileError:
		return "编译失败，可在题目页运行代码查看编译诊断。"
	case StatusSystemError:
		return "判题服务异常，请稍后重试。"
	default:
		return ""
	}
}
