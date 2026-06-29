package services

// ResumeData is the typed Go struct that mirrors the Claude extraction JSON schema.
// Every field maps directly to what Claude returns.
type ResumeData struct {
	FullName        string            `json:"full_name"`
	Headline        string            `json:"headline"`
	Summary         string            `json:"summary"`
	Email           *string           `json:"email"`
	Phone           *string           `json:"phone"`
	Location        *string           `json:"location"`
	LinkedInURL     *string           `json:"linkedin_url"`
	GithubURL       *string           `json:"github_url"`
	WebsiteURL      *string           `json:"website_url"`
	CareerType      string            `json:"career_type"`
	SuggestedTheme  string            `json:"suggested_theme"`
	Experience      []ExperienceItem  `json:"experience"`
	Education       []EducationItem   `json:"education"`
	Skills          []SkillGroup      `json:"skills"`
	Projects        []ProjectItem     `json:"projects"`
	Certifications  []CertItem        `json:"certifications"`
	Awards          []AwardItem       `json:"awards"`
	Publications    []PublicationItem `json:"publications"`
	Volunteer       []VolunteerItem   `json:"volunteer"`
	Languages       []LanguageItem    `json:"languages"`
	Interests       []string          `json:"interests"`
	CustomSections  []CustomSection   `json:"custom_sections"`
}

type ExperienceItem struct {
	Title       string   `json:"title"`
	Company     string   `json:"company"`
	Location    *string  `json:"location"`
	StartDate   *string  `json:"start_date"`
	EndDate     *string  `json:"end_date"`
	Description *string  `json:"description"`
	Bullets     []string `json:"bullets"`
	URL         *string  `json:"url"`
}

type EducationItem struct {
	Degree      string  `json:"degree"`
	Institution string  `json:"institution"`
	Location    *string `json:"location"`
	StartDate   *string `json:"start_date"`
	EndDate     *string `json:"end_date"`
	Description *string `json:"description"`
	Grade       *string `json:"grade"`
}

type SkillGroup struct {
	Category string   `json:"category"`
	Items    []string `json:"items"`
}

type ProjectItem struct {
	Name         string   `json:"name"`
	Description  string   `json:"description"`
	URL          *string  `json:"url"`
	StartDate    *string  `json:"start_date"`
	EndDate      *string  `json:"end_date"`
	Technologies []string `json:"technologies"`
	Bullets      []string `json:"bullets"`
}

type CertItem struct {
	Name         string  `json:"name"`
	Issuer       *string `json:"issuer"`
	Date         *string `json:"date"`
	URL          *string `json:"url"`
	CredentialID *string `json:"credential_id"`
}

type AwardItem struct {
	Title       string  `json:"title"`
	Issuer      *string `json:"issuer"`
	Date        *string `json:"date"`
	Description *string `json:"description"`
}

type PublicationItem struct {
	Title       string  `json:"title"`
	Publisher   *string `json:"publisher"`
	Date        *string `json:"date"`
	URL         *string `json:"url"`
	Description *string `json:"description"`
}

type VolunteerItem struct {
	Role         string  `json:"role"`
	Organization string  `json:"organization"`
	StartDate    *string `json:"start_date"`
	EndDate      *string `json:"end_date"`
	Description  *string `json:"description"`
}

type LanguageItem struct {
	Language    string `json:"language"`
	Proficiency string `json:"proficiency"`
}

type CustomSection struct {
	Title string `json:"title"`
	Items []struct {
		Title       *string `json:"title"`
		Description *string `json:"description"`
	} `json:"items"`
}

// DefaultSectionOrder defines the default display order for each career type.
// Sections not listed here appear at the end in the order Claude returned them.
var DefaultSectionOrder = map[string][]string{
	"developer":  {"experience", "skills", "projects", "education", "certifications", "awards"},
	"designer":   {"experience", "projects", "skills", "education", "certifications", "awards"},
	"creative":   {"experience", "projects", "awards", "skills", "education"},
	"corporate":  {"experience", "education", "skills", "certifications", "awards", "publications"},
	"academic":   {"education", "publications", "experience", "awards", "certifications"},
	"healthcare": {"experience", "education", "certifications", "skills", "awards"},
	"education":  {"experience", "education", "certifications", "publications", "skills"},
	"marketing":  {"experience", "projects", "skills", "education", "certifications", "awards"},
	"finance":    {"experience", "education", "certifications", "skills", "awards"},
	"legal":      {"experience", "education", "publications", "awards", "certifications"},
	"general":    {"experience", "education", "skills", "projects", "certifications", "awards"},
}
