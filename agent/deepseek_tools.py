def build_deepseek_tools() -> list[dict]:
    return [
        {
            "type": "function",
            "function": {
                "name": "user_ac_history",
                "strict": True,
                "description": "Get the user's solved history, including solved count and solved problem ids.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "user_id": {
                            "type": "integer",
                            "description": "User id",
                        }
                    },
                    "required": ["user_id"],
                    "additionalProperties": False,
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": "user_failed_submissions",
                "strict": True,
                "description": "Get the user's recent failed submissions.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "user_id": {
                            "type": "integer",
                            "description": "User id",
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Maximum number of failed submissions to return",
                            "minimum": 1,
                            "maximum": 20,
                        },
                    },
                    "required": ["user_id", "limit"],
                    "additionalProperties": False,
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": "user_tag_stats",
                "strict": True,
                "description": "Get the user's training statistics grouped by tag.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "user_id": {
                            "type": "integer",
                            "description": "User id",
                        }
                    },
                    "required": ["user_id"],
                    "additionalProperties": False,
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": "candidate_problems",
                "strict": True,
                "description": "Get candidate problems by explicit tags and exclude solved problem ids.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "tags": {
                            "type": "array",
                            "description": "Target tags for rule-based candidate retrieval",
                            "items": {
                                "type": "string",
                                "description": "Problem tag",
                            },
                        },
                        "exclude_ids": {
                            "type": "array",
                            "description": "Problem ids to exclude",
                            "items": {
                                "type": "integer",
                                "description": "Problem id",
                            },
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Maximum number of candidate problems to return",
                            "minimum": 1,
                            "maximum": 20,
                        },
                    },
                    "required": ["tags", "exclude_ids", "limit"],
                    "additionalProperties": False,
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": "semantic_candidate_problems",
                "strict": True,
                "description": "Retrieve semantically related candidate problems from the vector index using a natural language query.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "query": {
                            "type": "string",
                            "description": "Natural language retrieval query",
                        },
                        "exclude_ids": {
                            "type": "array",
                            "description": "Problem ids to exclude",
                            "items": {
                                "type": "integer",
                                "description": "Problem id",
                            },
                        },
                        "limit": {
                            "type": "integer",
                            "description": "Maximum number of semantic candidate problems to return",
                            "minimum": 1,
                            "maximum": 20,
                        },
                    },
                    "required": ["query", "exclude_ids", "limit"],
                    "additionalProperties": False,
                },
            },
        },
        {
            "type": "function",
            "function": {
                "name": "finish_study_plan",
                "strict": True,
                "description": "Finish and return the final structured study plan.",
                "parameters": {
                    "type": "object",
                    "properties": {
                        "weak_tags": {
                            "type": "array",
                            "description": "The user's current weak tags",
                            "items": {
                                "type": "string",
                                "description": "Weak tag",
                            },
                        },
                        "recommended_problems": {
                            "type": "array",
                            "description": "Recommended problem list",
                            "items": {
                                "type": "object",
                                "properties": {
                                    "problem_id": {
                                        "type": "integer",
                                        "description": "Problem id",
                                    },
                                    "title": {
                                        "type": "string",
                                        "description": "Problem title",
                                    },
                                    "reason": {
                                        "type": "string",
                                        "description": "Why this problem is recommended",
                                    },
                                },
                                "required": ["problem_id", "title", "reason"],
                                "additionalProperties": False,
                            },
                        },
                        "study_plan_summary": {
                            "type": "string",
                            "description": "Short study plan summary",
                        },
                    },
                    "required": ["weak_tags", "recommended_problems", "study_plan_summary"],
                    "additionalProperties": False,
                },
            },
        },
    ]
