package avid

import "regexp"

// PatternConfig holds configuration for pattern matching
type PatternConfig struct {
	// Special site patterns
	FC2Pattern      *regexp.Regexp
	HeydougaPattern *regexp.Regexp
	GetchuPattern   *regexp.Regexp
	GyuttoPattern   *regexp.Regexp
	
	// JAV studio patterns
	StandardPattern  *regexp.Regexp
	NumberedPattern  *regexp.Regexp
	ExtendedPattern  *regexp.Regexp
	
	// Utility patterns
	DomainPattern   *regexp.Regexp
	CJKPattern      *regexp.Regexp
}

// DefaultPatterns returns the default pattern configuration
func DefaultPatterns() *PatternConfig {
	return &PatternConfig{
		// FC2: FC2-PPV-1234567 or FC2-1234567
		FC2Pattern: regexp.MustCompile(`(?i)FC2[^A-Z\d]{0,5}(PPV[^A-Z\d]{0,5})?(\d{5,7})`),
		
		// HEYDOUGA: HEYDOUGA-4017-123
		HeydougaPattern: regexp.MustCompile(`(?i)(HEYDOUGA)[-_]*(\d{4})[-_]0?(\d{3,5})`),
		
		// GETCHU: GETCHU-1234567
		GetchuPattern: regexp.MustCompile(`(?i)GETCHU[-_]*(\d+)`),
		
		// GYUTTO: GYUTTO-123456
		GyuttoPattern: regexp.MustCompile(`(?i)GYUTTO-(\d+)`),
		
		// Standard JAV: ABC-123, ABCD-123
		StandardPattern: regexp.MustCompile(`(?i)([A-Z]{2,10})[-_]?(\d{2,5})`),
		
		// Numbered series: 300MIUM-001, 259LUXU-123
		NumberedPattern: regexp.MustCompile(`(?i)(\d{3}[A-Z]{2,8})[-_]?(\d{3,5})`),
		
		// Extended codes: SSIS-123A
		ExtendedPattern: regexp.MustCompile(`(?i)([A-Z]{2,10})[-_]?(\d{2,5})[A-Z]?`),
		
		// Domain removal
		DomainPattern: regexp.MustCompile(`(?i)\w{3,10}\.(COM|NET|APP|XYZ)`),
		
		// CJK character detection
		CJKPattern: regexp.MustCompile(`^[\u4e00-\u9fff\u3040-\u309f\u30a0-\u30ff\s\u3000！-～]+$`),
	}
}

// KnownStudioPrefixes contains known JAV studio prefixes
var KnownStudioPrefixes = map[string]string{
	// Major studios
	"SNIS":   "S1 NO.1 STYLE",
	"SSNI":   "S1 NO.1 STYLE",
	"SSIS":   "S1 NO.1 STYLE",
	"SOE":    "S1 NO.1 STYLE",
	"SONE":   "S1 NO.1 STYLE",
	
	"IPZ":    "IdeaPocket",
	"IPX":    "IdeaPocket", 
	"IDEA":   "IdeaPocket",
	
	"MIDE":   "MOODYZ",
	"MIDV":   "MOODYZ",
	"MDB":    "MOODYZ",
	
	"STARS":  "SOD Create",
	"STAR":   "SOD Create",
	"SDAB":   "SOD Create",
	
	"JUL":    "Madonna",
	"JUQ":    "Madonna",
	"JUX":    "Madonna",
	
	"PPPD":   "OPPAI",
	"PPSD":   "OPPAI",
	
	"ABP":    "Prestige",
	"ABW":    "Prestige",
	"ABS":    "Prestige",
	
	// Amateur series
	"GANA":   "Nampa TV",
	"SIRO":   "Shiroto TV", 
	"ARA":    "Ara",
	"MIUM":   "Mium",
	"259LUXU": "Luxury TV",
	"LUXU":   "Luxury TV",
	"DCV":    "Document TV",
	"HHL":    "Hunter",
	
	// FC2 and amateur
	"FC2":    "FC2",
}

// InvalidPrefixes contains prefixes that should not be recognized as JAV codes
var InvalidPrefixes = []string{
	// Technical terms
	"HTTP", "HTTPS", "WWW", "FTP", "COM", "NET", "ORG", "TV", "JP",
	
	// File formats
	"DVD", "BD", "CD", "MP", "AVI", "MKV", "MOV", "WMV", "MP4",
	
	// Video codecs
	"X264", "X265", "H264", "H265", "HEVC", "AVC",
	
	// Quality indicators
	"HD", "FHD", "UHD", "4K", "8K", "SD", "HQ", "LQ",
	"720P", "1080P", "2160P", "480P", "240P",
	
	// Common words
	"THE", "AND", "OR", "FOR", "WITH", "BY", "OF", "IN", "ON", "AT",
	
	// Years (to avoid matching year patterns)
	"2010", "2011", "2012", "2013", "2014", "2015", "2016", "2017", "2018", "2019",
	"2020", "2021", "2022", "2023", "2024", "2025",
}

// SpecialPatterns contains patterns for special ID formats
var SpecialPatterns = map[string]*regexp.Regexp{
	// FC2 variations
	"FC2_STANDARD": regexp.MustCompile(`(?i)FC2[-_\s]*PPV[-_\s]*(\d{6,7})`),
	"FC2_SHORT":    regexp.MustCompile(`(?i)FC2[-_\s]*(\d{6,7})`),
	
	// Carib variations
	"CARIB": regexp.MustCompile(`(?i)(CARIB|CARIBBEANCOM)[-_\s]*(\d{6})[-_](\d{3})`),
	
	// 1Pondo variations
	"PONDO": regexp.MustCompile(`(?i)(1PONDO|PONDO)[-_\s]*(\d{6})[-_](\d{3})`),
	
	// Heyzo variations  
	"HEYZO": regexp.MustCompile(`(?i)HEYZO[-_\s]*(\d{4})`),
	
	// Tokyo Hot variations
	"TOKYOHOT": regexp.MustCompile(`(?i)(TOKYO[-_\s]*HOT)[-_\s]*(N|K|RED)?[-_\s]*(\d{4})`),
	
	// Pacopacomama variations
	"PACOPACOMAMA": regexp.MustCompile(`(?i)PACOPACOMAMA[-_\s]*(\d{6})[-_](\d{3})`),
	
	// Muramura variations
	"MURAMURA": regexp.MustCompile(`(?i)MURAMURA[-_\s]*(\d{6})[-_](\d{3})`),
}

// AmateurSeries contains known amateur/素人 series prefixes
var AmateurSeries = []string{
	"GANA", "SIRO", "ARA", "MIUM", "HHL", "DCV", "LUXU", "259LUXU",
	"SWEET", "CUTE", "ORE", "MGS", "BEAV", "JKSR", "OKYH", "KTKP",
	"KTKL", "KTKC", "KTKT", "KTKZ", "KTKB", "KTKX", "KTKY", "KTDS",
	"KTKQ", "KTKW", "KTKV", "KTKG", "KTKE", "KTKS", "KTKF", "KTKA",
	"230ORE", "230ORETD", "261ARA", "200GANA", "277DCV", "229SCUTE",
	"300MIUM", "300MAAN", "390JAC", "428SUKE", "336KBI", "398CON",
}

// IsAmateurSeries checks if the given ID belongs to an amateur series
func IsAmateurSeries(id string) bool {
	id = regexp.MustCompile(`[-_]`).ReplaceAllString(id, "")
	id = regexp.MustCompile(`\d+`).ReplaceAllString(id, "")
	
	for _, series := range AmateurSeries {
		if id == series {
			return true
		}
	}
	return false
}