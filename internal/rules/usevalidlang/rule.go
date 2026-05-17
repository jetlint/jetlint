// Package usevalidlang implements use-valid-lang: the `lang`
// attribute on <html> should be a valid BCP-47 tag (ISO 639 language,
// optional ISO 15924 script, optional ISO 3166 region). A bogus tag
// is silently ignored by AT and the user never hears the right
// pronunciation.
package usevalidlang

import (
	"strings"

	wrapperchecker "github.com/microsoft/typescript-go/pkg/checker"

	"github.com/jetlint/jetlint/internal/engine"
	"github.com/jetlint/jetlint/internal/rules/internal/jsxutil"
)

const id = "use-valid-lang"

func New() engine.Rule { return rule{} }

type rule struct{}

func (rule) Meta() engine.Meta { return engine.Meta{ID: id} }

func (rule) Handlers() map[wrapperchecker.Kind]engine.Handler {
	return map[wrapperchecker.Kind]engine.Handler{
		wrapperchecker.KindJsxOpeningElement:     visit,
		wrapperchecker.KindJsxSelfClosingElement: visit,
	}
}

// ISO 639-1 two-letter language codes. ISO 639-2/3 adds many more
// three-letter codes; we accept any 2-3 lowercase string that isn't
// obviously bogus by deferring to the curated set for length-3.
var lang2 = map[string]bool{
	"aa": true, "ab": true, "ae": true, "af": true, "ak": true, "am": true,
	"an": true, "ar": true, "as": true, "av": true, "ay": true, "az": true,
	"ba": true, "be": true, "bg": true, "bh": true, "bi": true, "bm": true,
	"bn": true, "bo": true, "br": true, "bs": true, "ca": true, "ce": true,
	"ch": true, "co": true, "cr": true, "cs": true, "cu": true, "cv": true,
	"cy": true, "da": true, "de": true, "dv": true, "dz": true, "ee": true,
	"el": true, "en": true, "eo": true, "es": true, "et": true, "eu": true,
	"fa": true, "ff": true, "fi": true, "fj": true, "fo": true, "fr": true,
	"fy": true, "ga": true, "gd": true, "gl": true, "gn": true, "gu": true,
	"gv": true, "ha": true, "he": true, "hi": true, "ho": true, "hr": true,
	"ht": true, "hu": true, "hy": true, "hz": true, "ia": true, "id": true,
	"ie": true, "ig": true, "ii": true, "ik": true, "io": true, "is": true,
	"it": true, "iu": true, "ja": true, "jv": true, "ka": true, "kg": true,
	"ki": true, "kj": true, "kk": true, "kl": true, "km": true, "kn": true,
	"ko": true, "kr": true, "ks": true, "ku": true, "kv": true, "kw": true,
	"ky": true, "la": true, "lb": true, "lg": true, "li": true, "ln": true,
	"lo": true, "lt": true, "lu": true, "lv": true, "mg": true, "mh": true,
	"mi": true, "mk": true, "ml": true, "mn": true, "mr": true, "ms": true,
	"mt": true, "my": true, "na": true, "nb": true, "nd": true, "ne": true,
	"ng": true, "nl": true, "nn": true, "no": true, "nr": true, "nv": true,
	"ny": true, "oc": true, "oj": true, "om": true, "or": true, "os": true,
	"pa": true, "pi": true, "pl": true, "ps": true, "pt": true, "qu": true,
	"rm": true, "rn": true, "ro": true, "ru": true, "rw": true, "sa": true,
	"sc": true, "sd": true, "se": true, "sg": true, "si": true, "sk": true,
	"sl": true, "sm": true, "sn": true, "so": true, "sq": true, "sr": true,
	"ss": true, "st": true, "su": true, "sv": true, "sw": true, "ta": true,
	"te": true, "tg": true, "th": true, "ti": true, "tk": true, "tl": true,
	"tn": true, "to": true, "tr": true, "ts": true, "tt": true, "tw": true,
	"ty": true, "ug": true, "uk": true, "ur": true, "uz": true, "ve": true,
	"vi": true, "vo": true, "wa": true, "wo": true, "xh": true, "yi": true,
	"yo": true, "za": true, "zh": true, "zu": true,
}

// ISO 15924 script codes — common subset. (Full list is ~200; we
// cover the ones in actual use.)
var scripts = map[string]bool{
	"Arab": true, "Armn": true, "Beng": true, "Cans": true, "Cher": true,
	"Cyrl": true, "Deva": true, "Ethi": true, "Geor": true, "Grek": true,
	"Gujr": true, "Guru": true, "Hang": true, "Hani": true, "Hans": true,
	"Hant": true, "Hebr": true, "Hira": true, "Hrkt": true, "Jpan": true,
	"Kana": true, "Khmr": true, "Knda": true, "Kore": true, "Laoo": true,
	"Latn": true, "Mlym": true, "Mong": true, "Mymr": true, "Orya": true,
	"Sinh": true, "Syrc": true, "Taml": true, "Telu": true, "Thaa": true,
	"Thai": true, "Tibt": true, "Vaii": true, "Yiii": true,
}

// ISO 3166-1 alpha-2 region codes.
var regions = map[string]bool{
	"AD": true, "AE": true, "AF": true, "AG": true, "AI": true, "AL": true,
	"AM": true, "AO": true, "AQ": true, "AR": true, "AS": true, "AT": true,
	"AU": true, "AW": true, "AX": true, "AZ": true, "BA": true, "BB": true,
	"BD": true, "BE": true, "BF": true, "BG": true, "BH": true, "BI": true,
	"BJ": true, "BL": true, "BM": true, "BN": true, "BO": true, "BQ": true,
	"BR": true, "BS": true, "BT": true, "BV": true, "BW": true, "BY": true,
	"BZ": true, "CA": true, "CC": true, "CD": true, "CF": true, "CG": true,
	"CH": true, "CI": true, "CK": true, "CL": true, "CM": true, "CN": true,
	"CO": true, "CR": true, "CU": true, "CV": true, "CW": true, "CX": true,
	"CY": true, "CZ": true, "DE": true, "DJ": true, "DK": true, "DM": true,
	"DO": true, "DZ": true, "EC": true, "EE": true, "EG": true, "EH": true,
	"ER": true, "ES": true, "ET": true, "FI": true, "FJ": true, "FK": true,
	"FM": true, "FO": true, "FR": true, "GA": true, "GB": true, "GD": true,
	"GE": true, "GF": true, "GG": true, "GH": true, "GI": true, "GL": true,
	"GM": true, "GN": true, "GP": true, "GQ": true, "GR": true, "GS": true,
	"GT": true, "GU": true, "GW": true, "GY": true, "HK": true, "HM": true,
	"HN": true, "HR": true, "HT": true, "HU": true, "ID": true, "IE": true,
	"IL": true, "IM": true, "IN": true, "IO": true, "IQ": true, "IR": true,
	"IS": true, "IT": true, "JE": true, "JM": true, "JO": true, "JP": true,
	"KE": true, "KG": true, "KH": true, "KI": true, "KM": true, "KN": true,
	"KP": true, "KR": true, "KW": true, "KY": true, "KZ": true, "LA": true,
	"LB": true, "LC": true, "LI": true, "LK": true, "LR": true, "LS": true,
	"LT": true, "LU": true, "LV": true, "LY": true, "MA": true, "MC": true,
	"MD": true, "ME": true, "MF": true, "MG": true, "MH": true, "MK": true,
	"ML": true, "MM": true, "MN": true, "MO": true, "MP": true, "MQ": true,
	"MR": true, "MS": true, "MT": true, "MU": true, "MV": true, "MW": true,
	"MX": true, "MY": true, "MZ": true, "NA": true, "NC": true, "NE": true,
	"NF": true, "NG": true, "NI": true, "NL": true, "NO": true, "NP": true,
	"NR": true, "NU": true, "NZ": true, "OM": true, "PA": true, "PE": true,
	"PF": true, "PG": true, "PH": true, "PK": true, "PL": true, "PM": true,
	"PN": true, "PR": true, "PS": true, "PT": true, "PW": true, "PY": true,
	"QA": true, "RE": true, "RO": true, "RS": true, "RU": true, "RW": true,
	"SA": true, "SB": true, "SC": true, "SD": true, "SE": true, "SG": true,
	"SH": true, "SI": true, "SJ": true, "SK": true, "SL": true, "SM": true,
	"SN": true, "SO": true, "SR": true, "SS": true, "ST": true, "SV": true,
	"SX": true, "SY": true, "SZ": true, "TC": true, "TD": true, "TF": true,
	"TG": true, "TH": true, "TJ": true, "TK": true, "TL": true, "TM": true,
	"TN": true, "TO": true, "TR": true, "TT": true, "TV": true, "TW": true,
	"TZ": true, "UA": true, "UG": true, "UM": true, "US": true, "UY": true,
	"UZ": true, "VA": true, "VC": true, "VE": true, "VG": true, "VI": true,
	"VN": true, "VU": true, "WF": true, "WS": true, "YE": true, "YT": true,
	"ZA": true, "ZM": true, "ZW": true,
}

func visit(ctx *engine.Context, el *wrapperchecker.Node) {
	if jsxutil.TagName(el) != "html" {
		return
	}
	attrs := jsxutil.AttributesNode(el)
	attr := jsxutil.FindAttribute(attrs, "lang")
	if attr == nil {
		return
	}
	v, ok := jsxutil.AttributeStringValue(attr)
	if !ok {
		return
	}
	if !isValidLang(v) {
		ctx.Report(attr, "lang=\""+v+"\" isn't a valid BCP-47 tag")
	}
}

func isValidLang(v string) bool {
	parts := strings.Split(v, "-")
	if len(parts) == 0 || len(parts) > 3 {
		return false
	}
	if !isLanguageCode(parts[0]) {
		return false
	}
	if len(parts) == 1 {
		return true
	}
	// Second part: script (4 letters) or region (2 letters / 3 digits).
	hasScript := false
	if isScript(parts[1]) {
		hasScript = true
	} else if !isRegion(parts[1]) {
		return false
	}
	if len(parts) == 2 {
		return true
	}
	// Third part: must be region if second was script.
	if hasScript {
		return isRegion(parts[2])
	}
	// Third part: a variant subtag (5-8 alphanumeric chars). Without a
	// full variant catalog we err strict and reject.
	return false
}

func isLanguageCode(s string) bool {
	if len(s) == 2 {
		return lang2[strings.ToLower(s)]
	}
	if len(s) != 3 {
		return false
	}
	// 3-letter language codes (ISO 639-2/3) are too numerous to embed.
	// Accept any 3 lowercase letters; AT will accept them.
	for _, r := range s {
		if r < 'a' || r > 'z' {
			return false
		}
	}
	return true
}

func isScript(s string) bool {
	if len(s) != 4 {
		return false
	}
	return scripts[s]
}

func isRegion(s string) bool {
	if len(s) == 2 {
		return regions[strings.ToUpper(s)]
	}
	// Numeric region: UN M.49 (3 digits).
	if len(s) != 3 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
