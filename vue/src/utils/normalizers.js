function toNumber(value, fallback = 0) {
  const numeric = Number(value)
  return Number.isFinite(numeric) ? numeric : fallback
}

export function normalizeTag(input = {}) {
  return {
    id: toNumber(input.ID ?? input.id),
    name: input.Name ?? input.name ?? '',
  }
}

export function normalizeProblem(input = {}) {
  return {
    id: toNumber(input.ID ?? input.id),
    title: input.Title ?? input.title ?? '',
    description: input.Description ?? input.description ?? '',
    timeLimit: toNumber(input.TimeLimit ?? input.time_limit, 1000),
    memoryLimit: toNumber(input.MemoryLimit ?? input.memory_limit, 256),
    submitCount: toNumber(input.SubmitCount ?? input.submit_count),
    acceptedCount: toNumber(input.AcceptedCount ?? input.accepted_count),
    isAc: Boolean(input.IsAC ?? input.is_ac),
    tags: (input.Tags ?? input.tags ?? []).map(normalizeTag),
  }
}

export function normalizeSubmission(input = {}) {
  return {
    id: toNumber(input.ID ?? input.id ?? input.submission_id),
    problemId: toNumber(input.ProblemID ?? input.problem_id),
    problemTitle: input.ProblemTitle ?? input.problem_title ?? '',
    language: input.Language ?? input.language ?? '',
    code: input.Code ?? input.code ?? '',
    status: input.Status ?? input.status ?? 'Pending',
    timeCost: toNumber(input.TimeCost ?? input.time_cost),
    memoryCost: toNumber(input.MemoryCost ?? input.memory_cost),
    actualOutput: input.ActualOutput ?? input.actual_output ?? '',
    createdAt: input.CreatedAt ?? input.created_at ?? '',
    updatedAt: input.UpdatedAt ?? input.updated_at ?? '',
  }
}

export function normalizeProfile(input = {}) {
  return {
    id: toNumber(input.ID ?? input.id),
    username: input.Username ?? input.username ?? '',
    role: toNumber(input.Role ?? input.role),
    solvedCount: toNumber(input.SolvedCount ?? input.solved_count),
    solvedList: (input.SolvedList ?? input.solved_list ?? []).map((item) => toNumber(item)),
  }
}

export function normalizeLeaderboardItem(input = {}) {
  return {
    rank: toNumber(input.Rank ?? input.rank ?? input.leaderboard),
    score: toNumber(input.Score ?? input.score),
    userId: toNumber(input.UserID ?? input.user_id),
    username: input.Username ?? input.username ?? 'Anonymous',
  }
}

export function normalizeTestCase(input = {}) {
  return {
    id: toNumber(input.ID ?? input.id),
    problemId: toNumber(input.ProblemID ?? input.problem_id),
    input: input.Input ?? input.input ?? '',
    expectedOutput: input.ExpectedOutput ?? input.expected_output ?? '',
    createdAt: input.CreatedAt ?? input.created_at ?? '',
  }
}

export function getAcceptanceRate(problem) {
  const submitCount = toNumber(problem?.submitCount)
  const acceptedCount = toNumber(problem?.acceptedCount)

  if (!submitCount) {
    return 0
  }

  return Math.round((acceptedCount / submitCount) * 100)
}
