package instagram

var (
	TimelineFeedReasons = []string{"cold_start_fetch", "warm_start_fetch", "pagination", "pull_to_refresh", "auto_refresh"}
	ReelsTrayReasons    = []string{"cold_start", "pull_to_refresh"}
)

type LoginResult struct {
	Success           bool
	UserID            int64
	Username          string
	TwoFactorRequired bool
	TwoFactorInfo     map[string]any
	ChallengeRequired bool
	ChallengeInfo     map[string]any
	Error             error
}

type WebLoginResponse struct {
	Authenticated     bool   `json:"authenticated"`
	User              bool   `json:"user"`
	UserID            string `json:"userId"`
	OneTapPrompt      bool   `json:"oneTapPrompt"`
	Status            string `json:"status"`
	Message           string `json:"message"`
	TwoFactorRequired bool   `json:"two_factor_required"`
	TwoFactorInfo     struct {
		TwoFactorIdentifier string `json:"two_factor_identifier"`
		Username            string `json:"username"`
	} `json:"two_factor_info"`
	Checkpoint struct {
		URL string `json:"url"`
	} `json:"checkpoint_url"`
	ErrorType string `json:"error_type"`
}
