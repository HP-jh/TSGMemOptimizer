package engine

type Profile struct {
	Name        string            `json:"name"`
	Label       string            `json:"label"`
	Description string            `json:"description"`
	BoostPIDs   map[string]int    `json:"boostPIDs"`   // process name -> priority boost
	ProtectPIDs []string          `json:"protectPIDs"` // never kill
	CleanLevel  string            `json:"cleanLevel"`
	AutoThreshold int             `json:"autoThreshold"`
}

var Profiles = map[string]Profile{
	"balanced": {
		Name: "balanced", Label: "均衡模式", Description: "平衡内存清理与系统稳定性",
		BoostPIDs: map[string]int{}, ProtectPIDs: []string{"tsgmemo", "TarsSecureGuard"},
		CleanLevel: "standard", AutoThreshold: 85,
	},
	"office": {
		Name: "office", Label: "文职模式", Description: "优先保障办公软件内存，提升文档处理流畅度",
		BoostPIDs: map[string]int{"wps": -5, "soffice": -5, "word": -5, "excel": -5},
		ProtectPIDs: []string{"wps", "soffice", "word", "excel", "tsgmemo"},
		CleanLevel: "light", AutoThreshold: 80,
	},
	"dev": {
		Name: "dev", Label: "开发模式", Description: "优先保障IDE与编译器，减少构建中断",
		BoostPIDs: map[string]int{"code": -5, "idea": -5, "goland": -5, "vscode": -5},
		ProtectPIDs: []string{"code", "idea", "goland", "vscode", "docker", "tsgmemo"},
		CleanLevel: "standard", AutoThreshold: 88,
	},
	"gaming": {
		Name: "gaming", Label: "游戏模式", Description: "释放最大内存给游戏进程",
		BoostPIDs: map[string]int{}, ProtectPIDs: []string{"tsgmemo"},
		CleanLevel: "deep", AutoThreshold: 75,
	},
	"ai": {
		Name: "ai", Label: "AI运行模式", Description: "为本地AI推理优化内存，保障模型加载稳定",
		BoostPIDs: map[string]int{"ollama": -10, "llama": -10, "python": -5},
		ProtectPIDs: []string{"ollama", "llama", "python", "tsgmemo", "TarsSecureGuard"},
		CleanLevel: "standard", AutoThreshold: 82,
	},
}

func GetProfile(name string) (Profile, bool) {
	p, ok := Profiles[name]
	return p, ok
}

func ProfileNames() []string {
	return []string{"balanced", "office", "dev", "gaming", "ai"}
}
