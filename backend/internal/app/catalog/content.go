package catalog

import (
	"context"
	"database/sql"
	"regexp"

	"jstihuo/internal/app/identity"
)

type ContentInput struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Body      string `json:"body_html"`
	Kind      string `json:"kind"`
	Published bool   `json:"published"`
	Sort      int    `json:"sort_order"`
}

type ContentUpdate struct {
	ContentInput
	Version int64 `json:"version"`
}

type Content struct {
	ContentUpdate
	ID string `json:"id"`
}

var contentSlug = regexp.MustCompile(`^[a-z0-9-]{1,64}$`)

// validateContent 用途：校验内容字段并复用商品正文的 HTML 白名单。
// 参数：in 完整内容快照。
// 返回值：清洗后快照；字段不合法返回 422，解析失败返回错误。
func validateContent(in ContentInput) (ContentInput, error) {
	validKind := in.Kind == "PAGE" || in.Kind == "ANNOUNCEMENT" || in.Kind == "BANNER"
	if !contentSlug.MatchString(in.Slug) || !length(in.Title, 1, 120) ||
		!length(in.Body, 0, 50000) || !validKind || in.Sort < 0 || in.Sort > 1000000 {
		return in, fault(422, "INVALID_ARGUMENT")
	}
	clean, err := Sanitize(in.Body)
	if err != nil {
		return in, err
	}
	if !length(clean, 0, 50000) {
		return in, fault(422, "INVALID_ARGUMENT")
	}
	in.Body = clean
	return in, nil
}

// ContentList 用途：读取固定降序游标的内容列表。
// 参数：ctx 上下文；f 分页、标题或 slug 搜索及发布状态；public 是否仅限已发布。
// 返回值：内容分页；非法筛选返回 400，数据库失败返回错误。
func (s *Service) ContentList(ctx context.Context, f Filter, public bool) (Page[Content], error) {
	if f.Limit < 1 || f.Limit > 100 || f.After < 0 || !length(f.Q, 0, 100) ||
		(f.Status != "" && f.Status != "PUBLISHED" && f.Status != "DRAFT") ||
		(public && f.Status == "DRAFT") {
		return Page[Content]{}, fault(400, "INVALID_ARGUMENT")
	}
	return listContent(ctx, s.DB, f, public)
}

// ContentGet 用途：按后台 ID 或公开 slug 读取内容。
// 参数：ctx 上下文；idOrSlug 后台 ID 或公开 slug；public 是否为公开查询。
// 返回值：内容；不存在或公开访问草稿返回 404，参数不合法返回 422。
func (s *Service) ContentGet(ctx context.Context, idOrSlug string, public bool) (Content, error) {
	if public {
		if !contentSlug.MatchString(idOrSlug) {
			return Content{}, fault(422, "INVALID_ARGUMENT")
		}
	} else if _, err := parseID(idOrSlug); err != nil {
		return Content{}, err
	}
	return loadContent(ctx, s.DB, idOrSlug, public)
}

// CreateContent 用途：创建草稿或直接发布自有内容。
// 参数：ctx 上下文；a 管理员；in 完整内容；key 幂等键；rid 请求标识。
// 返回值：清洗后持久化内容；无权限返回 403，重复 slug 或幂等冲突返回 409。
// 事务：内容、幂等结果和不含正文的审计一起提交。
func (s *Service) CreateContent(ctx context.Context, a identity.Actor, in ContentInput, key, rid string) (Content, error) {
	clean, err := validateContent(in)
	if err != nil {
		return Content{}, err
	}
	ref, err := s.mutate(ctx, a, "createContent", key, "content.write", rid, in,
		func(tx *sql.Tx) (Ref, error) { return insertContent(ctx, tx, clean) })
	if err != nil {
		return Content{}, err
	}
	return s.ContentGet(ctx, ref.ID, false)
}

// UpdateContent 用途：按版本替换内容，包括发布与撤下。
// 参数：ctx 上下文；a 管理员；id 内容 ID；in 完整快照；key 幂等键；rid 请求标识。
// 返回值：持久化内容；版本或 slug 冲突返回 409，不存在返回 404。
// 事务：版本比较、内容、幂等与审计原子提交，失败不改变发布状态。
func (s *Service) UpdateContent(ctx context.Context, a identity.Actor, id string, in ContentUpdate, key, rid string) (Content, error) {
	if _, err := parseID(id); err != nil {
		return Content{}, err
	}
	if in.Version < 1 || in.Version >= 9007199254740991 {
		return Content{}, fault(422, "INVALID_ARGUMENT")
	}
	clean, err := validateContent(in.ContentInput)
	if err != nil {
		return Content{}, err
	}
	input := struct {
		ID string
		ContentUpdate
	}{id, in}
	ref, err := s.mutate(ctx, a, "updateContent", key, "content.write", rid, input,
		func(tx *sql.Tx) (Ref, error) {
			return updateContent(ctx, tx, id, ContentUpdate{clean, in.Version})
		})
	if err != nil {
		return Content{}, err
	}
	return s.ContentGet(ctx, ref.ID, false)
}
