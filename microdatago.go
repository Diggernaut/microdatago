package microdatago

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"reflect"
	"strings"
	"unicode"

	"github.com/diggernaut/cast"
	"github.com/diggernaut/goquery"
)

// Parser is an HTML parser that extracts microdata
type Parser struct {
	r         io.Reader
	Microdata []map[string]interface{}
	baseURL   *url.URL
}

// JSON converts the data object to JSON
func (p *Parser) JSON() ([]byte, error) {
	b, err := json.Marshal(p.Microdata)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// NewParser creates a new parser for extracting microdata
// r is a reader over an HTML document
// baseURL is the base URL for resolving relative URLs
func NewParser(r io.Reader, baseURL *url.URL) *Parser {
	return &Parser{
		r:       r,
		baseURL: baseURL,
	}
}

// Parse the document and return an error if there is any issue with parsing
func (p *Parser) Parse() error {
	dom, err := goquery.NewDocumentFromReader(p.r)
	if err != nil {
		return err
	}
	// Doing DOM clean-up, filter out nodes without microdata markup
	selector := dom.Find("[itemprop],[itemscope]")
	selector2 := dom.Find("*")
	selector2.Each(func(i int, s2 *goquery.Selection) {
		found := false
		selector.Each(func(i int, s *goquery.Selection) {
			if s2.IsSelection(s) {
				found = true
				return
			}
			sel := s2.HasSelection(s)
			if sel.Length() > 0 {
				found = true
				return
			}
		})
		if !found {
			s2.Remove()
		}
	})
	selector = dom.Find(":not([itemprop],[itemscope])")
	for selector.Length() > 0 {
		selector.Each(func(i int, s *goquery.Selection) {
			if _, ok := s.Attr("itemprop"); ok {
				return
			}
			if _, ok := s.Attr("itemscope"); ok {
				return
			}
			html, _ := s.Html()
			s.ReplaceWithHtml(html)
		})
		selector = dom.Find(":not([itemprop],[itemscope])")
	}

	// Parsing DOM with microdata to the object
	selector = dom.ChildrenFiltered("*")
	selector.Each(func(i int, s *goquery.Selection) {
		item := make(map[string]interface{})
		p.extractItem(s, item)
		p.Microdata = append(p.Microdata, item)
	})

	return nil
}

func (p *Parser) extractItem(selector *goquery.Selection, item map[string]interface{}) {
	var fieldname string
	if itemprop, ok := selector.Attr("itemprop"); ok {
		fieldname = itemprop
	} else {
		if itemtype, ok := selector.Attr("itemtype"); ok {
			itemtypeComponents := strings.Split(itemtype, "/")
			if len(itemtypeComponents) > 0 {
				fieldname = itemtypeComponents[len(itemtypeComponents)-1]
			} else {
				fieldname = itemtype
			}
		}
	}
	sel := selector.ChildrenFiltered("*")
	if sel.Length() > 0 {
		newItem := make(map[string]interface{})
		if href, ok := selector.Attr("href"); ok {
			if href != "" {
				relURL, err := url.Parse(href)
				if err == nil {
					url := p.baseURL.ResolveReference(relURL)
					newItem["url"] = url.String()
				}
			}
		}
		sel.Each(func(i int, s *goquery.Selection) {
			p.extractItem(s, newItem)
		})
		if currentValue, ok := item[fieldname]; ok {
			if it := reflect.TypeOf(currentValue).Kind(); it == reflect.Slice {
				anArray := cast.ToSlice(currentValue)
				anArray = append(anArray, newItem)
				item[fieldname] = anArray
			} else {
				var anArray []interface{}
				anArray = append(anArray, currentValue, newItem)
				item[fieldname] = anArray
			}
		} else {
			item[fieldname] = newItem
		}
	} else {
		value := ""
		if content, ok := selector.Attr("content"); ok {
			if content != "" {
				value = content
			}
		}
		if value == "" {
			if content, ok := selector.Attr("href"); ok {
				if content != "" {
					relURL, err := url.Parse(content)
					if err == nil {
						url := p.baseURL.ResolveReference(relURL)
						value = url.String()
					}
				}
			}
		}
		if value == "" {
			if content, ok := selector.Attr("src"); ok {
				if content != "" {
					relURL, err := url.Parse(content)
					if err == nil {
						url := p.baseURL.ResolveReference(relURL)
						value = url.String()
					}
				}
			}
		}
		if value == "" {
			text := strings.TrimSpace(stringMinifier(selector.Text()))
			value = text
		}
		if currentValue, ok := item[fieldname]; ok {
			if it := reflect.TypeOf(currentValue).Kind(); it == reflect.Slice {
				anArray := cast.ToSlice(currentValue)
				anArray = append(anArray, value)
				item[fieldname] = anArray
			} else {
				var anArray []interface{}
				anArray = append(anArray, currentValue, value)
				item[fieldname] = anArray
			}
		} else {
			item[fieldname] = value
		}
	}
}

func stringMinifier(in string) string {
	buf := bytes.NewBufferString("")
	white := false
	for _, c := range in {
		if unicode.IsSpace(c) {
			if !white {
				buf.WriteString(" ")
			}
			white = true
		} else {
			buf.WriteString(string(c))
			white = false
		}
	}
	return buf.String()
}
