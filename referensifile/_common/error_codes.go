// error_codes.go — compile-time const enum untuk educational_errors.
//
// FIX #31 (effekdomino.md): caller code sebelumnya pakai string literal
// langsung (misal `GetEducationalError("ERR_X", ...)`). Typo silent —
// "ERR_MISSING_ARGU" pernah dipakai padahal seed kanonikal "ERR_MISSING_ARGUMENT",
// lookup miss → fallback generic, warga AI ngga belajar recovery pattern.
//
// Solusi: const block ini = single source of truth Go-side. Caller harus
// pakai const (compile-time check), BUKAN string literal.
//
// Cara pakai:
//
//	import braindb "github.com/teetah2402/flowork/brain/db"
//	msg := braindb.GetEducationalError(workspace, braindb.ErrMissingArgument, ...)
//
// Boot-time `ValidateErrorCodeConsts(db)` cross-check setiap const di sini
// punya row di DB seed (panic kalau drift). Plus warn kalau seed punya code
// yang ngga punya const (developer lupa update file ini).

package db

import (
	"database/sql"
	"fmt"
)

// ErrCode adalah identifier educational error. Pakai const di bawah,
// JANGAN string literal — typo lookup miss = warga ngga belajar.
type ErrCode = string

// Const block — sinkron dengan seedEducationalErrors di educational_errors_seed.go.
// Tambah const baru sini setiap tambah entry seed (boot-time validator
// flag drift).
const (
	ErrAmnesiaHistory          ErrCode = "ERR_AMNESIA_HISTORY"
	ErrAPIKeyMissing           ErrCode = "ERR_API_KEY_MISSING"
	ErrBFTDAGBlocked           ErrCode = "ERR_BFT_DAG_BLOCKED"
	ErrBFTInvalidProposal      ErrCode = "ERR_BFT_INVALID_PROPOSAL"
	ErrBlindGuess              ErrCode = "ERR_BLIND_GUESS"
	ErrBrowserNavigateFailed   ErrCode = "ERR_BROWSER_NAVIGATE_FAILED"
	ErrBrowserStartFailed      ErrCode = "ERR_BROWSER_START_FAILED"
	ErrBudgetExceeded          ErrCode = "ERR_BUDGET_EXCEEDED"
	ErrBudgetGuardBlocked      ErrCode = "ERR_BUDGET_GUARD_BLOCKED"
	ErrCodemapUnavailable      ErrCode = "ERR_CODEMAP_UNAVAILABLE"
	ErrCommandTimeout          ErrCode = "ERR_COMMAND_TIMEOUT"
	ErrConstitutionBreach      ErrCode = "ERR_CONSTITUTION_BREACH"
	ErrCreativityStagnant      ErrCode = "ERR_CREATIVITY_STAGNANT"
	ErrDirectoryWalkFailed     ErrCode = "ERR_DIRECTORY_WALK_FAILED"
	ErrDistillWastefulTeacher  ErrCode = "ERR_DISTILL_WASTEFUL_TEACHER"
	ErrDNSResolveFailed        ErrCode = "ERR_DNS_RESOLVE_FAILED"
	ErrEditAmbiguousMatch      ErrCode = "ERR_EDIT_AMBIGUOUS_MATCH"
	ErrEditTargetNotFound      ErrCode = "ERR_EDIT_TARGET_NOT_FOUND"
	ErrEmptyLLMResponse        ErrCode = "ERR_EMPTY_LLM_RESPONSE"
	ErrFileLocked              ErrCode = "ERR_FILE_LOCKED"
	ErrFileOpenFailed          ErrCode = "ERR_FILE_OPEN_FAILED"
	ErrFileReadFailed          ErrCode = "ERR_FILE_READ_FAILED"
	ErrFileWriteFailed         ErrCode = "ERR_FILE_WRITE_FAILED"
	ErrGitHookRejected         ErrCode = "ERR_GIT_HOOK_REJECTED"
	ErrDoctrineRecite          ErrCode = "ERR_DOCTRINE_RECITE"
	ErrHaluNoProof             ErrCode = "ERR_HALU_NO_PROOF"
	ErrHTTPError               ErrCode = "ERR_HTTP_ERROR"
	ErrInvalidDispatchTarget   ErrCode = "ERR_INVALID_DISPATCH_TARGET"
	ErrInvalidMemoryType       ErrCode = "ERR_INVALID_MEMORY_TYPE"
	ErrInvalidPeriod           ErrCode = "ERR_INVALID_PERIOD"
	ErrInvalidTodoStatus       ErrCode = "ERR_INVALID_TODO_STATUS"
	ErrKarmaLow                ErrCode = "ERR_KARMA_LOW"
	ErrKernelAPIFailed         ErrCode = "ERR_KERNEL_API_FAILED"
	ErrKernelCommFailed        ErrCode = "ERR_KERNEL_COMM_FAILED"
	ErrLLMProviderError        ErrCode = "ERR_LLM_PROVIDER_ERROR"
	ErrLocalModelFailed        ErrCode = "ERR_LOCAL_MODEL_FAILED"
	ErrMCPInjectionBlocked     ErrCode = "ERR_MCP_INJECTION_BLOCKED"
	ErrMCPNotWhitelisted       ErrCode = "ERR_MCP_NOT_WHITELISTED"
	ErrMCPServerMissing        ErrCode = "ERR_MCP_SERVER_MISSING"
	ErrMissingArgument         ErrCode = "ERR_MISSING_ARGUMENT"
	ErrMusicAssetMissing       ErrCode = "ERR_MUSIC_ASSET_MISSING"
	ErrMusicDistributorLogin   ErrCode = "ERR_MUSIC_DISTRIBUTOR_LOGIN"
	ErrNetworkError            ErrCode = "ERR_NETWORK_ERROR"
	ErrPanicLoop               ErrCode = "ERR_PANIC_LOOP"
	ErrPatternInvalid          ErrCode = "ERR_PATTERN_INVALID"
	ErrPermissionDeniedDaemon  ErrCode = "ERR_PERMISSION_DENIED_DAEMON"
	ErrPlanScopeRestricted     ErrCode = "ERR_PLAN_SCOPE_RESTRICTED"
	ErrProtectedCoreBlocked    ErrCode = "ERR_PROTECTED_CORE_BLOCKED"
	ErrSearchProviderError     ErrCode = "ERR_SEARCH_PROVIDER_ERROR"
	ErrShellSafetyBlocked      ErrCode = "ERR_SHELL_SAFETY_BLOCKED"
	ErrSocmedAuthMissing       ErrCode = "ERR_SOCMED_AUTH_MISSING"
	ErrSSRFBlocked             ErrCode = "ERR_SSRF_BLOCKED"
	ErrSymlinkAttack           ErrCode = "ERR_SYMLINK_ATTACK"
	ErrTaskNotFound            ErrCode = "ERR_TASK_NOT_FOUND"
	ErrTodoOneInProgress       ErrCode = "ERR_TODO_ONE_IN_PROGRESS"
	ErrTokenWaste              ErrCode = "ERR_TOKEN_WASTE"
	ErrToolNotFound            ErrCode = "ERR_TOOL_NOT_FOUND"
	ErrTooManyRedirects        ErrCode = "ERR_TOO_MANY_REDIRECTS"
	ErrUnsupportedScheme       ErrCode = "ERR_UNSUPPORTED_SCHEME"
	ErrVoiceModelDenied        ErrCode = "ERR_VOICE_MODEL_DENIED"
	ErrVoteReasonRequired      ErrCode = "ERR_VOTE_REASON_REQUIRED"
	ErrWargaInactive           ErrCode = "ERR_WARGA_INACTIVE"
	ErrWorkspaceNotFound       ErrCode = "ERR_WORKSPACE_NOT_FOUND"
)

// allErrorConsts — daftar const di file ini, dipakai validator boot-time
// untuk cross-check vs DB seed. Tambah baris baru setiap tambah const di
// atas (compile-time pair).
var allErrorConsts = []ErrCode{
	ErrAmnesiaHistory, ErrAPIKeyMissing, ErrBFTDAGBlocked, ErrBFTInvalidProposal,
	ErrBlindGuess, ErrBrowserNavigateFailed, ErrBrowserStartFailed, ErrBudgetExceeded,
	ErrBudgetGuardBlocked, ErrCodemapUnavailable, ErrCommandTimeout, ErrConstitutionBreach,
	ErrCreativityStagnant, ErrDirectoryWalkFailed, ErrDistillWastefulTeacher, ErrDNSResolveFailed,
	ErrEditAmbiguousMatch, ErrEditTargetNotFound, ErrEmptyLLMResponse, ErrFileLocked,
	ErrFileOpenFailed, ErrFileReadFailed, ErrFileWriteFailed, ErrGitHookRejected,
	ErrHaluNoProof, ErrHTTPError, ErrInvalidDispatchTarget, ErrInvalidMemoryType,
	ErrInvalidPeriod, ErrInvalidTodoStatus, ErrKarmaLow, ErrKernelAPIFailed,
	ErrKernelCommFailed, ErrLLMProviderError, ErrLocalModelFailed, ErrMCPInjectionBlocked,
	ErrMCPNotWhitelisted, ErrMCPServerMissing, ErrMissingArgument, ErrMusicAssetMissing,
	ErrMusicDistributorLogin, ErrNetworkError, ErrPanicLoop, ErrPatternInvalid,
	ErrPermissionDeniedDaemon, ErrPlanScopeRestricted, ErrProtectedCoreBlocked, ErrSearchProviderError,
	ErrShellSafetyBlocked, ErrSocmedAuthMissing, ErrSSRFBlocked, ErrSymlinkAttack,
	ErrTaskNotFound, ErrTodoOneInProgress, ErrTokenWaste, ErrToolNotFound,
	ErrTooManyRedirects, ErrUnsupportedScheme, ErrVoiceModelDenied, ErrVoteReasonRequired,
	ErrWargaInactive, ErrWorkspaceNotFound,
}

// ValidateErrorCodeConsts cross-check sinkron antara const block (Go) dan
// seedEducationalErrors slice (DB seed). Return error kalau drift:
//   - const ada di Go tapi ngga ada row di seed (developer lupa seed)
//   - seed punya code yang ngga ada const (developer lupa update file ini)
//
// Dipanggil dari SeedEducationalErrors atau test boot-time. Idempotent.
func ValidateErrorCodeConsts(db *sql.DB) error {
	seedSet := make(map[string]bool, len(seedEducationalErrors))
	for _, s := range seedEducationalErrors {
		seedSet[s.ErrorCode] = true
	}
	constSet := make(map[string]bool, len(allErrorConsts))
	for _, c := range allErrorConsts {
		constSet[c] = true
	}

	var missingInSeed []string
	for c := range constSet {
		if !seedSet[c] {
			missingInSeed = append(missingInSeed, c)
		}
	}
	var missingInConst []string
	for c := range seedSet {
		if !constSet[c] {
			missingInConst = append(missingInConst, c)
		}
	}

	if len(missingInSeed) > 0 || len(missingInConst) > 0 {
		return fmt.Errorf(
			"educational error code drift: %d const tanpa seed %v, %d seed tanpa const %v",
			len(missingInSeed), missingInSeed, len(missingInConst), missingInConst,
		)
	}
	return nil
}
