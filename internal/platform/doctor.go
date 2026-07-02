package platform

type DoctorCheck struct {
	Name    string
	OK      bool
	Message string
}

func CredentialChecks(provider Provider, env map[string]string) []DoctorCheck {
	switch provider {
	case ProviderGitHub:
		if env["GITHUB_TOKEN"] == "" {
			return []DoctorCheck{{Name: "github token", OK: false, Message: "GITHUB_TOKEN is not set"}}
		}
		return []DoctorCheck{{Name: "github token", OK: true, Message: "GITHUB_TOKEN is set"}}
	case ProviderFeishu:
		checks := []DoctorCheck{}
		if env["FEISHU_APP_ID"] == "" {
			checks = append(checks, DoctorCheck{Name: "feishu app id", OK: false, Message: "FEISHU_APP_ID is not set"})
		} else {
			checks = append(checks, DoctorCheck{Name: "feishu app id", OK: true, Message: "FEISHU_APP_ID is set"})
		}
		if env["FEISHU_APP_SECRET"] == "" {
			checks = append(checks, DoctorCheck{Name: "feishu app secret", OK: false, Message: "FEISHU_APP_SECRET is not set"})
		} else {
			checks = append(checks, DoctorCheck{Name: "feishu app secret", OK: true, Message: "FEISHU_APP_SECRET is set"})
		}
		return checks
	default:
		return []DoctorCheck{{Name: "platform", OK: false, Message: "unsupported platform"}}
	}
}
