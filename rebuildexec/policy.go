package rebuildexec

import (
	"encoding/json"
	"encoding/xml"
	"log"
)

const (
	cmpAllow       = "0"
	cmpSanitise    = "1"
	cmpDisallow    = "2"
	cmpAllowXml    = "allow"
	cmpSanitiseXml = "sanitise"
	cmpDisallowXml = "disallow"
)

type policy struct {
	PolicyId                    string                 `json:"PolicyId,omitempty"`
	ContentManagementFlags      ContentManagementFlags `json:"ContentManagementFlags"`
	UnprocessableFileTypeAction int                    `json:"UnprocessableFileTypeAction,omitempty"`
	GlasswallBlockedFilesAction int                    `json:"GlasswallBlockedFilesAction,omitempty"`
	NcfsRoutingUrl              string                 `json:"NcfsRoutingUrl,omitempty"`
}

type ContentManagementFlags struct {
	PdfContentManagement        Pdfcontentmanagement        `json:"PdfContentManagement" xml:"pdfConfig"`
	WordContentManagement       Wordcontentmanagement       `json:"WordContentManagement" xml:"wordConfig"`
	PowerPointContentManagement Powerpointcontentmanagement `json:"PowerPointContentManagement" xml:"pptConfig"`
	ExcelContentManagement      Excelcontentmanagement      `json:"ExcelContentManagement" xml:"xlsConfig"`
}
type XmlConfig struct {
	XMLName                     xml.Name                    `xml:"config"`
	PdfContentManagement        Pdfcontentmanagement        `xml:"pdfConfig"`
	WordContentManagement       Wordcontentmanagement       `xml:"wordConfig"`
	PowerPointContentManagement Powerpointcontentmanagement `xml:"pptConfig"`
	ExcelContentManagement      Excelcontentmanagement      `xml:"xlsConfig"`
}

/*

type XmlConfig struct {
	XMLName                     xml.Name                    `xml:"config"`
	PdfContentManagement        Pdfcontentmanagement        `xml:"pdfConfig"`
	WordContentManagement       Wordcontentmanagement       `xml:"wordConfig"`
	PowerPointContentManagement Powerpointcontentmanagement `xml:"pptConfig"`
	ExcelContentManagement      Excelcontentmanagement      `xml:"xlsConfig"`
}
*/

type Pdfcontentmanagement struct {
	Watermark          json.Number `json:"-" xml:"watermark"`
	Metadata           json.Number `json:"Metadata" xml:"metadata"`
	InternalHyperlinks json.Number `json:"InternalHyperlinks" xml:"internal_hyperlinks"`
	ExternalHyperlinks json.Number `json:"ExternalHyperlinks" xml:"external_hyperlinks"`
	EmbeddedFiles      json.Number `json:"EmbeddedFiles" xml:"embedded_files"`
	EmbeddedImages     json.Number `json:"EmbeddedImages" xml:"embedded_images"`
	Javascript         json.Number `json:"Javascript" xml:"javascript"`
	Acroform           json.Number `json:"Acroform" xml:"acroform"`
	ActionsAll         json.Number `json:"ActionsAll" xml:"actions_all"`
}

type Excelcontentmanagement struct {
	Metadata            json.Number `json:"Metadata" xml:"metadata"`
	InternalHyperlinks  json.Number `json:"InternalHyperlinks" xml:"internal_hyperlinks"`
	ExternalHyperlinks  json.Number `json:"ExternalHyperlinks" xml:"external_hyperlinks"`
	EmbeddedFiles       json.Number `json:"EmbeddedFiles" xml:"embedded_files"`
	EmbeddedImages      json.Number `json:"EmbeddedImages" xml:"embedded_images"`
	Dynamicdataexchange json.Number `json:"DynamicDataExchange" xml:"dynamic_data_exchange"`
	Macros              json.Number `json:"Macros" xml:"macros"`
	Reviewcomments      json.Number `json:"ReviewComments" xml:"review_comments"`
}
type Powerpointcontentmanagement struct {
	Metadata           json.Number `json:"Metadata" xml:"metadata"`
	InternalHyperlinks json.Number `json:"InternalHyperlinks" xml:"internal_hyperlinks"`
	ExternalHyperlinks json.Number `json:"ExternalHyperlinks" xml:"external_hyperlinks"`
	EmbeddedFiles      json.Number `json:"EmbeddedFiles" xml:"embedded_files"`
	EmbeddedImages     json.Number `json:"EmbeddedImages" xml:"embedded_images"`
	Macros             json.Number `json:"Macros" xml:"macros"`
	Reviewcomments     json.Number `json:"ReviewComments" xml:"review_comments"`
}
type Wordcontentmanagement struct {
	Metadata            json.Number `json:"Metadata" xml:"metadata"`
	InternalHyperlinks  json.Number `json:"InternalHyperlinks" xml:"internal_hyperlinks"`
	ExternalHyperlinks  json.Number `json:"ExternalHyperlinks" xml:"external_hyperlinks"`
	EmbeddedFiles       json.Number `json:"EmbeddedFiles" xml:"embedded_files"`
	EmbeddedImages      json.Number `json:"EmbeddedImages" xml:"embedded_images"`
	Dynamicdataexchange json.Number `json:"DynamicDataExchange" xml:"dynamic_data_exchange"`
	Macros              json.Number `json:"Macros" xml:"macros"`
	Reviewcomments      json.Number `json:"ReviewComments" xml:"review_comments"`
}

type Config struct {
	XMLName   xml.Name `xml:"config"`
	Text      string   `xml:",chardata"`
	PdfConfig struct {
		Text               string `xml:",chardata"`
		Watermark          string `xml:"watermark"`
		Acroform           string `xml:"acroform"`
		Metadata           string `xml:"metadata"`
		Javascript         string `xml:"javascript"`
		ActionsAll         string `xml:"actions_all"`
		EmbeddedFiles      string `xml:"embedded_files"`
		InternalHyperlinks string `xml:"internal_hyperlinks"`
		ExternalHyperlinks string `xml:"external_hyperlinks"`
	} `xml:"pdfConfig"`
	WordConfig struct {
		Text               string `xml:",chardata"`
		Macros             string `xml:"macros"`
		Metadata           string `xml:"metadata"`
		EmbeddedFiles      string `xml:"embedded_files"`
		ReviewComments     string `xml:"review_comments"`
		InternalHyperlinks string `xml:"internal_hyperlinks"`
		ExternalHyperlinks string `xml:"external_hyperlinks"`
	} `xml:"wordConfig"`
	PptConfig struct {
		Text               string `xml:",chardata"`
		Macros             string `xml:"macros"`
		Metadata           string `xml:"metadata"`
		EmbeddedFiles      string `xml:"embedded_files"`
		ReviewComments     string `xml:"review_comments"`
		InternalHyperlinks string `xml:"internal_hyperlinks"`
		ExternalHyperlinks string `xml:"external_hyperlinks"`
	} `xml:"pptConfig"`
	XlsConfig struct {
		Text               string `xml:",chardata"`
		Macros             string `xml:"macros"`
		Metadata           string `xml:"metadata"`
		EmbeddedFiles      string `xml:"embedded_files"`
		ReviewComments     string `xml:"review_comments"`
		InternalHyperlinks string `xml:"internal_hyperlinks"`
		ExternalHyperlinks string `xml:"external_hyperlinks"`
	} `xml:"xlsConfig"`
}

func cmpJsontoXml(b []byte) []byte {
	cmp := ContentManagementFlags{}
	if err := json.Unmarshal(b, &cmp); err != nil {
		log.Fatal(err)
	}
	cmpXml := XmlConfig{
		XMLName:                     xml.Name{},
		PdfContentManagement:        cmp.PdfContentManagement,
		WordContentManagement:       cmp.WordContentManagement,
		PowerPointContentManagement: cmp.PowerPointContentManagement,
		ExcelContentManagement:      cmp.ExcelContentManagement,
	}
	cmpXml.PdfContentManagement.cmpNumToStr()
	xmlB, err := xml.MarshalIndent(cmpXml, "", "   ")
	if err != nil {
		log.Fatal(err)
	}
	xmlB = append([]byte(xml.Header), xmlB...)
	return xmlB

}

func (p *Pdfcontentmanagement) cmpNumToStr() {
	p.Metadata = cmpNumToStr(p.Metadata)
	p.InternalHyperlinks = cmpNumToStr(p.InternalHyperlinks)
	p.ExternalHyperlinks = cmpNumToStr(p.ExternalHyperlinks)
	p.EmbeddedFiles = cmpNumToStr(p.EmbeddedFiles)
	p.EmbeddedImages = cmpNumToStr(p.EmbeddedImages)
	p.Javascript = cmpNumToStr(p.Javascript)
	p.Acroform = cmpNumToStr(p.Acroform)
	p.ActionsAll = cmpNumToStr(p.ActionsAll)

}

func (p *Excelcontentmanagement) cmpNumToStr() {
	p.Metadata = cmpNumToStr(p.Metadata)
	p.InternalHyperlinks = cmpNumToStr(p.InternalHyperlinks)
	p.ExternalHyperlinks = cmpNumToStr(p.ExternalHyperlinks)
	p.EmbeddedFiles = cmpNumToStr(p.EmbeddedFiles)
	p.EmbeddedImages = cmpNumToStr(p.EmbeddedImages)
	p.Dynamicdataexchange = cmpNumToStr(p.Dynamicdataexchange)
	p.Macros = cmpNumToStr(p.Macros)
	p.Reviewcomments = cmpNumToStr(p.Reviewcomments)

}

func (p *Powerpointcontentmanagement) cmpNumToStr() {
	p.Metadata = cmpNumToStr(p.Metadata)
	p.InternalHyperlinks = cmpNumToStr(p.InternalHyperlinks)
	p.ExternalHyperlinks = cmpNumToStr(p.ExternalHyperlinks)
	p.EmbeddedFiles = cmpNumToStr(p.EmbeddedFiles)
	p.EmbeddedImages = cmpNumToStr(p.EmbeddedImages)

	p.Macros = cmpNumToStr(p.Macros)
	p.Reviewcomments = cmpNumToStr(p.Reviewcomments)

}
func (p *Wordcontentmanagement) cmpNumToStr() {
	p.Metadata = cmpNumToStr(p.Metadata)
	p.InternalHyperlinks = cmpNumToStr(p.InternalHyperlinks)
	p.ExternalHyperlinks = cmpNumToStr(p.ExternalHyperlinks)
	p.EmbeddedFiles = cmpNumToStr(p.EmbeddedFiles)
	p.EmbeddedImages = cmpNumToStr(p.EmbeddedImages)
	p.Dynamicdataexchange = cmpNumToStr(p.Dynamicdataexchange)
	p.Macros = cmpNumToStr(p.Macros)
	p.Reviewcomments = cmpNumToStr(p.Reviewcomments)

}
func cmpNumToStr(v json.Number) json.Number {
	switch v {
	case cmpAllow:
		return cmpAllowXml
	case cmpSanitise:
		return cmpSanitiseXml
	case cmpDisallow:
		return cmpDisallowXml
	}
	return ""
}

func cmpStrToNum(v json.Number) json.Number {
	switch v {
	case cmpAllowXml:
		return cmpAllow
	case cmpSanitiseXml:
		return cmpSanitise
	case cmpDisallowXml:
		return cmpDisallow
	}
	return ""
}

/*
0: Allow the content

1: Sanitise (Default) the content

2: Disallow the content


<xs:enumeration value="sanitise"/>
<xs:enumeration value="allow"/>
<xs:enumeration value="disallow"/>
*/
