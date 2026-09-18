package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"gojo/config"
	"gojo/infrastructure/cache"
	"gojo/infrastructure/mysql"
	"gojo/infrastructure/search"
	problemdto "gojo/internal/problem/dto"
	problemmodel "gojo/internal/problem/model"
	problemrepo "gojo/internal/problem/repository"
	problemservice "gojo/internal/problem/service"
	"gojo/internal/syncer"
)

type seedProblem struct {
	Title       string
	Description string
	TimeLimit   int
	MemoryLimit int
	Tags        []string
	TestCases   []problemdto.TestCaseRequest
}

func main() {
	fmt.Println("seeding problem set...")

	config.InitConfig()
	mysql.InitDB()
	cache.InitRedis()
	search.InitElasticsearch()

	ctx := context.Background()

	problemRepository := problemrepo.NewProblemRepository()
	searchRepository := problemrepo.NewProblemSearchRepository()
	syncManager := syncer.NewManager(problemRepository, searchRepository)
	tagSvc := problemservice.NewTagService(problemrepo.NewTagRepository(), syncManager)
	problemSvc := problemservice.NewProblemService(problemRepository, searchRepository, syncManager)

	tagIDs, err := ensureTags(ctx, tagSvc, seedTags())
	if err != nil {
		log.Fatalf("ensure tags failed: %v", err)
	}

	existingTitles, err := loadExistingTitles(ctx)
	if err != nil {
		log.Fatalf("load existing titles failed: %v", err)
	}

	createdCount := 0
	skippedCount := 0

	for _, item := range seedProblems() {
		key := normalizeKey(item.Title)
		if existingTitles[key] {
			fmt.Printf("skip existing problem: %s\n", item.Title)
			skippedCount++
			continue
		}

		req := problemdto.ProblemRequest{
			Title:       item.Title,
			Description: strings.TrimSpace(item.Description),
			TimeLimit:   item.TimeLimit,
			MemoryLimit: item.MemoryLimit,
			TestCases:   item.TestCases,
			TagIDs:      collectTagIDs(tagIDs, item.Tags),
		}

		problem, err := problemSvc.CreateProblem(ctx, req)
		if err != nil {
			log.Fatalf("create problem %q failed: %v", item.Title, err)
		}

		existingTitles[key] = true
		createdCount++
		fmt.Printf("created problem #%d: %s\n", problem.ID, item.Title)
	}

	if err := problemSvc.SyncAllProblemsToES(ctx); err != nil {
		log.Printf("sync all problems to elasticsearch failed: %v", err)
	}

	fmt.Printf("seed completed: created=%d skipped=%d\n", createdCount, skippedCount)
}

func ensureTags(ctx context.Context, svc *problemservice.TagService, names []string) (map[string]uint, error) {
	existing, err := svc.GetTagList(ctx)
	if err != nil {
		return nil, err
	}

	tagIDs := make(map[string]uint, len(existing)+len(names))
	for _, tag := range existing {
		tagIDs[normalizeKey(tag.Name)] = tag.ID
	}

	for _, name := range names {
		key := normalizeKey(name)
		if _, ok := tagIDs[key]; ok {
			continue
		}

		tag, err := svc.CreateTag(ctx, name)
		if err != nil {
			return nil, err
		}
		tagIDs[key] = tag.ID
		fmt.Printf("created tag #%d: %s\n", tag.ID, tag.Name)
	}

	return tagIDs, nil
}

func loadExistingTitles(ctx context.Context) (map[string]bool, error) {
	var problems []problemmodel.Problem
	if err := mysql.DB.WithContext(ctx).Select("title").Find(&problems).Error; err != nil {
		return nil, err
	}

	result := make(map[string]bool, len(problems))
	for _, problem := range problems {
		result[normalizeKey(problem.Title)] = true
	}
	return result, nil
}

func collectTagIDs(tagIDs map[string]uint, names []string) []uint {
	ids := make([]uint, 0, len(names))
	for _, name := range names {
		id, ok := tagIDs[normalizeKey(name)]
		if !ok {
			log.Fatalf("missing tag id for tag %q", name)
		}
		ids = append(ids, id)
	}
	return ids
}

func normalizeKey(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func seedTags() []string {
	return []string{
		"数组",
		"字符串",
		"哈希表",
		"模拟",
		"排序",
		"前缀和",
		"二分",
		"双指针",
		"滑动窗口",
		"栈",
		"队列",
		"链表",
		"树",
		"二叉树",
		"图论",
		"BFS",
		"DFS",
		"动态规划",
		"贪心",
		"数学",
		"位运算",
	}
}

func seedProblems() []seedProblem {
	return []seedProblem{
		{
			Title: "A+B Problem",
			Description: `
给定两个整数 a 和 b，输出它们的和。

输入格式：
一行，包含两个整数 a 和 b。

输出格式：
输出一个整数，表示 a+b。
`,
			Tags: []string{"数组", "数学", "模拟"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "1 2\n", ExpectedOutput: "3\n"},
				{Input: "-5 8\n", ExpectedOutput: "3\n"},
				{Input: "0 0\n", ExpectedOutput: "0\n"},
			},
		},
		{
			Title: "括号是否合法",
			Description: `
给定一个只包含 ()[]{} 的字符串，判断括号是否完全匹配。

输入格式：
一行字符串 s。

输出格式：
若合法输出 true，否则输出 false。
`,
			Tags: []string{"字符串", "栈"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "()[]{}\n", ExpectedOutput: "true\n"},
				{Input: "([)]\n", ExpectedOutput: "false\n"},
				{Input: "{[()]}\n", ExpectedOutput: "true\n"},
			},
		},
		{
			Title: "Two Sum 下标对",
			Description: `
给定长度为 n 的数组和目标值 target，找到两个下标 i<j，使得 a[i]+a[j]=target。
保证恰好存在一组解。

输入格式：
第一行一个整数 n。
第二行 n 个整数。
第三行一个整数 target。

输出格式：
输出两个下标（从 0 开始），用空格分隔。
`,
			Tags: []string{"数组", "哈希表"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "4\n2 7 11 15\n9\n", ExpectedOutput: "0 1\n"},
				{Input: "3\n3 2 4\n6\n", ExpectedOutput: "1 2\n"},
				{Input: "2\n3 3\n6\n", ExpectedOutput: "0 1\n"},
			},
		},
		{
			Title: "有序数组二分查找",
			Description: `
给定一个升序数组和目标值 target，输出 target 首次出现的位置；若不存在输出 -1。

输入格式：
第一行一个整数 n。
第二行 n 个升序整数。
第三行一个整数 target。

输出格式：
输出一个整数下标。
`,
			Tags: []string{"数组", "二分"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "6\n1 2 2 2 5 9\n2\n", ExpectedOutput: "1\n"},
				{Input: "5\n1 3 5 7 9\n4\n", ExpectedOutput: "-1\n"},
				{Input: "1\n8\n8\n", ExpectedOutput: "0\n"},
			},
		},
		{
			Title: "有序数组平方后排序",
			Description: `
给定一个非递减数组，返回每个元素平方后的非递减结果。

输入格式：
第一行一个整数 n。
第二行 n 个整数。

输出格式：
输出 n 个整数，按非递减顺序排列。
`,
			Tags: []string{"数组", "双指针", "排序"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "5\n-4 -1 0 3 10\n", ExpectedOutput: "0 1 9 16 100\n"},
				{Input: "4\n-7 -3 2 3\n", ExpectedOutput: "4 9 9 49\n"},
				{Input: "1\n5\n", ExpectedOutput: "25\n"},
			},
		},
		{
			Title: "最大子数组和",
			Description: `
给定一个整数数组，求连续子数组的最大和。

输入格式：
第一行一个整数 n。
第二行 n 个整数。

输出格式：
输出最大子数组和。
`,
			Tags: []string{"数组", "动态规划"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "9\n-2 1 -3 4 -1 2 1 -5 4\n", ExpectedOutput: "6\n"},
				{Input: "1\n1\n", ExpectedOutput: "1\n"},
				{Input: "5\n-5 -2 -8 -1 -3\n", ExpectedOutput: "-1\n"},
			},
		},
		{
			Title: "爬楼梯",
			Description: `
每次可以爬 1 级或 2 级台阶，求爬到第 n 级台阶有多少种不同方法。

输入格式：
一行一个整数 n。

输出格式：
输出方案数。
`,
			Tags: []string{"动态规划", "数学"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "2\n", ExpectedOutput: "2\n"},
				{Input: "5\n", ExpectedOutput: "8\n"},
				{Input: "8\n", ExpectedOutput: "34\n"},
			},
		},
		{
			Title: "合并区间",
			Description: `
给定 n 个区间 [l,r]，合并所有有交集的区间。

输入格式：
第一行一个整数 n。
接下来 n 行，每行两个整数 l 和 r。

输出格式：
第一行输出合并后的区间个数 m。
接下来 m 行，每行两个整数，按左端点升序输出。
`,
			Tags: []string{"数组", "排序"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "4\n1 3\n2 6\n8 10\n15 18\n", ExpectedOutput: "3\n1 6\n8 10\n15 18\n"},
				{Input: "2\n1 4\n4 5\n", ExpectedOutput: "1\n1 5\n"},
				{Input: "3\n1 2\n3 4\n5 6\n", ExpectedOutput: "3\n1 2\n3 4\n5 6\n"},
			},
		},
		{
			Title: "最长无重复子串长度",
			Description: `
给定字符串 s，求不含重复字符的最长连续子串长度。

输入格式：
一行字符串 s。

输出格式：
输出一个整数。
`,
			Tags: []string{"字符串", "哈希表", "滑动窗口"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "abcabcbb\n", ExpectedOutput: "3\n"},
				{Input: "bbbbb\n", ExpectedOutput: "1\n"},
				{Input: "pwwkew\n", ExpectedOutput: "3\n"},
			},
		},
		{
			Title: "前缀和区间求和",
			Description: `
给定长度为 n 的数组和 q 次查询，每次查询区间 [l,r] 的元素和。

输入格式：
第一行两个整数 n 和 q。
第二行 n 个整数。
接下来 q 行，每行两个整数 l 和 r（0-based）。

输出格式：
每次查询输出一行区间和。
`,
			Tags: []string{"数组", "前缀和"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "5 3\n1 2 3 4 5\n0 2\n1 3\n2 4\n", ExpectedOutput: "6\n9\n12\n"},
				{Input: "4 2\n5 -2 7 1\n0 0\n0 3\n", ExpectedOutput: "5\n11\n"},
				{Input: "3 1\n10 20 30\n1 1\n", ExpectedOutput: "20\n"},
			},
		},
		{
			Title: "逆波兰表达式求值",
			Description: `
给定一个后缀表达式，支持 + - * / 四则运算，除法向 0 截断。

输入格式：
第一行一个整数 n，表示 token 数量。
第二行 n 个 token，以空格分隔。

输出格式：
输出表达式结果。
`,
			Tags: []string{"栈", "字符串", "模拟"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "5\n2 1 + 3 *\n", ExpectedOutput: "9\n"},
				{Input: "5\n4 13 5 / +\n", ExpectedOutput: "6\n"},
				{Input: "13\n10 6 9 3 + -11 * / * 17 + 5 +\n", ExpectedOutput: "22\n"},
			},
		},
		{
			Title: "反转链表",
			Description: `
给定一个单链表，输出反转后的链表。

输入格式：
第一行一个整数 n。
第二行 n 个整数，表示链表节点值。

输出格式：
输出反转后的链表节点值，用空格分隔。
`,
			Tags: []string{"链表"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "5\n1 2 3 4 5\n", ExpectedOutput: "5 4 3 2 1\n"},
				{Input: "1\n7\n", ExpectedOutput: "7\n"},
				{Input: "4\n9 8 7 6\n", ExpectedOutput: "6 7 8 9\n"},
			},
		},
		{
			Title: "合并两个有序链表",
			Description: `
给定两个非递减链表，输出合并后的非递减链表。

输入格式：
第一行一个整数 n。
第二行 n 个整数。
第三行一个整数 m。
第四行 m 个整数。

输出格式：
输出合并后的链表，用空格分隔。
`,
			Tags: []string{"链表", "双指针"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "3\n1 2 4\n3\n1 3 4\n", ExpectedOutput: "1 1 2 3 4 4\n"},
				{Input: "0\n\n1\n0\n", ExpectedOutput: "0\n"},
				{Input: "2\n5 7\n2\n1 6\n", ExpectedOutput: "1 5 6 7\n"},
			},
		},
		{
			Title: "二叉树层序遍历",
			Description: `
给定一棵二叉树的层序数组表示，-1 表示空节点，输出每一层的节点值。

输入格式：
第一行一个整数 n。
第二行 n 个整数，按层序给出。

输出格式：
第一行输出层数。
接下来每行输出一层的节点值，节点之间用空格分隔。
`,
			Tags: []string{"树", "二叉树", "BFS", "队列"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "7\n3 9 20 -1 -1 15 7\n", ExpectedOutput: "3\n3\n9 20\n15 7\n"},
				{Input: "1\n1\n", ExpectedOutput: "1\n1\n"},
				{Input: "3\n1 -1 2\n", ExpectedOutput: "2\n1\n2\n"},
			},
		},
		{
			Title: "岛屿数量",
			Description: `
给定一个由字符 0 和 1 构成的网格，求岛屿数量。上下左右相邻的 1 属于同一座岛。

输入格式：
第一行两个整数 n 和 m。
接下来 n 行，每行一个长度为 m 的字符串。

输出格式：
输出岛屿数量。
`,
			Tags: []string{"图论", "DFS", "BFS"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "4 5\n11000\n11000\n00100\n00011\n", ExpectedOutput: "3\n"},
				{Input: "3 3\n111\n010\n111\n", ExpectedOutput: "1\n"},
				{Input: "2 4\n0000\n0000\n", ExpectedOutput: "0\n"},
			},
		},
		{
			Title: "迷宫最短路",
			Description: `
给定一个只包含 0 和 1 的网格，0 表示可走，1 表示障碍。
从左上角走到右下角，求最短步数；若无法到达输出 -1。

输入格式：
第一行两个整数 n 和 m。
接下来 n 行，每行 m 个字符。

输出格式：
输出最短步数。
`,
			Tags: []string{"图论", "BFS", "队列"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "3 3\n000\n010\n000\n", ExpectedOutput: "4\n"},
				{Input: "2 2\n01\n10\n", ExpectedOutput: "-1\n"},
				{Input: "4 4\n0000\n1110\n0000\n0000\n", ExpectedOutput: "6\n"},
			},
		},
		{
			Title: "零钱兑换最少硬币数",
			Description: `
给定若干硬币面值和目标金额 amount，求凑出 amount 所需的最少硬币数；若无法凑出输出 -1。

输入格式：
第一行两个整数 n 和 amount。
第二行 n 个正整数，表示硬币面值。

输出格式：
输出最少硬币数。
`,
			Tags: []string{"动态规划"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "3 11\n1 2 5\n", ExpectedOutput: "3\n"},
				{Input: "1 3\n2\n", ExpectedOutput: "-1\n"},
				{Input: "4 27\n2 5 10 1\n", ExpectedOutput: "4\n"},
			},
		},
		{
			Title: "分发饼干",
			Description: `
每个孩子有一个胃口值，每块饼干有一个尺寸。
一块饼干只能给一个孩子，且当饼干尺寸不小于孩子胃口值时孩子才会满足。
求最多能满足多少个孩子。

输入格式：
第一行一个整数 n。
第二行 n 个整数，表示孩子胃口值。
第三行一个整数 m。
第四行 m 个整数，表示饼干尺寸。

输出格式：
输出最多满足的孩子数量。
`,
			Tags: []string{"贪心", "排序"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "3\n1 2 3\n2\n1 1\n", ExpectedOutput: "1\n"},
				{Input: "2\n1 2\n3\n1 2 3\n", ExpectedOutput: "2\n"},
				{Input: "4\n2 3 4 5\n3\n1 2 3\n", ExpectedOutput: "2\n"},
			},
		},
		{
			Title: "多数元素",
			Description: `
给定一个长度为 n 的数组，保证存在一个元素出现次数严格大于 n/2，求这个元素。

输入格式：
第一行一个整数 n。
第二行 n 个整数。

输出格式：
输出多数元素。
`,
			Tags: []string{"数组", "贪心", "数学"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "3\n3 2 3\n", ExpectedOutput: "3\n"},
				{Input: "7\n2 2 1 1 1 2 2\n", ExpectedOutput: "2\n"},
				{Input: "1\n9\n", ExpectedOutput: "9\n"},
			},
		},
		{
			Title: "从 0 到 n 的比特位计数",
			Description: `
给定一个非负整数 n，输出从 0 到 n 的每个整数的二进制中 1 的个数。

输入格式：
一行一个整数 n。

输出格式：
输出 n+1 个整数，用空格分隔。
`,
			Tags: []string{"动态规划", "位运算"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "2\n", ExpectedOutput: "0 1 1\n"},
				{Input: "5\n", ExpectedOutput: "0 1 1 2 1 2\n"},
				{Input: "0\n", ExpectedOutput: "0\n"},
			},
		},
		{
			Title: "最小覆盖子串长度",
			Description: `
给定两个字符串 s 和 t，求 s 中包含 t 所有字符（含重复次数）的最短子串长度；若不存在输出 0。

输入格式：
第一行字符串 s。
第二行字符串 t。

输出格式：
输出最短长度。
`,
			Tags: []string{"字符串", "哈希表", "滑动窗口"},
			TestCases: []problemdto.TestCaseRequest{
				{Input: "ADOBECODEBANC\nABC\n", ExpectedOutput: "4\n"},
				{Input: "a\naa\n", ExpectedOutput: "0\n"},
				{Input: "aa\naa\n", ExpectedOutput: "2\n"},
			},
		},
	}
}
