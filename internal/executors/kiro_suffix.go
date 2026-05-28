// Kiro synthetic-suffix helpers + agentic system prompt. The Kiro upstream
// doesn't actually accept "-thinking" / "-agentic" / "-thinking-agentic"
// model ids — those are local routing variants. This file owns the strip /
// detection / prompt logic so the executor can route the right shape
// upstream while letting users pick the variant by name.

package executors

import "strings"

const (
	KiroAgenticSuffix         = "-agentic"
	KiroThinkingSuffix        = "-thinking"
	KiroThinkingBudgetDefault = 16000
)

// KiroAgenticSystemPrompt is prepended to every Kiro request that resolves
// to an "-agentic" variant. The chunked-write rules below ship verbatim
// because Kiro's 2-3min upstream timeout makes single large writes fail
// silently — having the model self-throttle is the only reliable fix.
const KiroAgenticSystemPrompt = `# CRITICAL: CHUNKED WRITE PROTOCOL (MANDATORY)

You MUST follow these rules for ALL file operations. Violation causes server timeouts and task failure.

## ABSOLUTE LIMITS
- **MAXIMUM 350 LINES** per single write/edit operation - NO EXCEPTIONS
- **RECOMMENDED 300 LINES** or less for optimal performance
- **NEVER** write entire files in one operation if >300 lines

## MANDATORY CHUNKED WRITE STRATEGY

### For NEW FILES (>300 lines total):
1. FIRST: Write initial chunk (first 250-300 lines) using write_to_file/fsWrite
2. THEN: Append remaining content in 250-300 line chunks using file append operations
3. REPEAT: Continue appending until complete

### For EDITING EXISTING FILES:
1. Use surgical edits (apply_diff/targeted edits) - change ONLY what's needed
2. NEVER rewrite entire files - use incremental modifications
3. Split large refactors into multiple small, focused edits

### For LARGE CODE GENERATION:
1. Generate in logical sections (imports, types, functions separately)
2. Write each section as a separate operation
3. Use append operations for subsequent sections

## EXAMPLES OF CORRECT BEHAVIOR

CORRECT: Writing a 600-line file
- Operation 1: Write lines 1-300 (initial file creation)
- Operation 2: Append lines 301-600

CORRECT: Editing multiple functions
- Operation 1: Edit function A
- Operation 2: Edit function B
- Operation 3: Edit function C

WRONG: Writing 500 lines in single operation -> TIMEOUT
WRONG: Rewriting entire file to change 5 lines -> TIMEOUT
WRONG: Generating massive code blocks without chunking -> TIMEOUT

## WHY THIS MATTERS
- Server has 2-3 minute timeout for operations
- Large writes exceed timeout and FAIL completely
- Chunked writes are FASTER and more RELIABLE
- Failed writes waste time and require retry

REMEMBER: When in doubt, write LESS per operation. Multiple small operations > one large operation.`

// IsKiroAgenticModel reports whether model carries the -agentic suffix
// (either alone or combined with -thinking).
func IsKiroAgenticModel(model string) bool {
	return strings.HasSuffix(model, KiroAgenticSuffix) ||
		strings.HasSuffix(model, KiroThinkingSuffix+KiroAgenticSuffix) ||
		strings.HasSuffix(model, KiroAgenticSuffix+KiroThinkingSuffix)
}

// IsKiroThinkingModel reports whether model carries the -thinking suffix.
func IsKiroThinkingModel(model string) bool {
	return strings.HasSuffix(model, KiroThinkingSuffix) ||
		strings.HasSuffix(model, KiroAgenticSuffix+KiroThinkingSuffix) ||
		strings.HasSuffix(model, KiroThinkingSuffix+KiroAgenticSuffix)
}

// ResolveKiroModel strips every synthetic suffix from model and returns the
// base id Kiro will actually accept upstream.
func ResolveKiroModel(model string) string {
	for {
		switch {
		case strings.HasSuffix(model, KiroAgenticSuffix):
			model = model[:len(model)-len(KiroAgenticSuffix)]
		case strings.HasSuffix(model, KiroThinkingSuffix):
			model = model[:len(model)-len(KiroThinkingSuffix)]
		default:
			return model
		}
	}
}
