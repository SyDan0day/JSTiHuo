package regions

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

//go:embed pca-code.json provenance.json LICENSE
var files embed.FS

type Region struct {
	Code   string `json:"code"`
	Parent string `json:"parent_code"`
	Name   string `json:"name"`
}
type node struct {
	Code     string `json:"code"`
	Name     string `json:"name"`
	Children []node `json:"children"`
}
type Dataset struct {
	ByCode   map[string]Region
	ByParent map[string][]Region
}

// Load 用途：读取嵌入的锁定版本地区数据并建立父子索引。
// 参数：无。
// 返回值：数据集或读取、格式、重复代码错误。
// 约束：不设区城市以市级代码表示全市，镇街须写入详细地址，不伪造县级代码。
func Load() (Dataset, error) {
	raw, e := files.ReadFile("pca-code.json")
	if e != nil {
		return Dataset{}, e
	}
	var nodes []node
	if e = json.Unmarshal(raw, &nodes); e != nil {
		return Dataset{}, e
	}
	d := Dataset{map[string]Region{}, map[string][]Region{}}
	var walk func([]node, string) error
	walk = func(nodes []node, parent string) error {
		for _, n := range nodes {
			if len(n.Code) > 6 {
				continue
			}
			if len(n.Code) != 2 && len(n.Code) != 4 && len(n.Code) != 6 {
				return fmt.Errorf("invalid region code")
			}
			code := n.Code + strings.Repeat("0", 6-len(n.Code))
			if _, exists := d.ByCode[code]; exists {
				return fmt.Errorf("duplicate region")
			}
			r := Region{code, parent, n.Name}
			d.ByCode[code] = r
			d.ByParent[parent] = append(d.ByParent[parent], r)
			if e := walk(n.Children, code); e != nil {
				return e
			}
			if len(n.Children) > 0 && len(d.ByParent[code]) == 0 {
				d.ByParent[code] = []Region{{Code: code, Parent: code, Name: n.Name + "（全市）"}}
			}
		}
		return nil
	}
	if e = walk(nodes, "000000"); e != nil {
		return d, e
	}
	for p, list := range d.ByParent {
		sort.Slice(list, func(i, j int) bool { return list[i].Code < list[j].Code })
		d.ByParent[p] = list
	}
	return d, nil
}

// Validate 用途：校验省、市、区三级归属。
// 参数：province 六位省份代码；city 六位城市代码。
// 参数：district 六位区县代码。
// 返回值：完整父子链合法时为 true。
// 约束：包括不设区城市的全市第三层。
func (d Dataset) Validate(province, city, district string) bool {
	p, ok := d.ByCode[province]
	if !ok || p.Parent != "000000" {
		return false
	}
	c, ok := d.ByCode[city]
	if !ok || c.Parent != province {
		return false
	}
	for _, a := range d.ByParent[city] {
		if a.Code == district {
			return true
		}
	}
	return false
}

// Label 用途：组合地区显示名称。
// 参数：province 六位省份代码；city 六位城市代码。
// 参数：district 六位区县代码。
// 返回值：地区名称字符串。
func (d Dataset) Label(province, city, district string) string {
	return d.ByCode[province].Name + " " + d.ByCode[city].Name + " " + d.ByCode[district].Name
}
