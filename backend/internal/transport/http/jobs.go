package transport

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"jstihuo/internal/app/identity"
	"jstihuo/internal/app/operations"
	rules "jstihuo/internal/domain/voucher"
)

// serveJobs 用途：处理文件任务创建、读取和导入确认。
// 参数：w 响应器；r 已分派的任务请求。
// 返回值：无，异步创建或确认返回 202，不把受理显示为完成。
func (h OperationsHandler) serveJobs(w http.ResponseWriter, r *http.Request) {
	permission := "file.read"
	if r.Method != "GET" {
		permission = "file.import"
		if r.URL.Path == "/api/v1/admin/export-jobs" {
			permission = "file.export"
		}
	}
	a, err := h.operationActor(r, "ADMIN", permission)
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	var result any
	status := 202
	key, rid := r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID")
	switch {
	case r.URL.Path == "/api/v1/admin/import-jobs" && r.Method == "POST":
		var in operations.ImportInput
		err = operationBody(w, r, &in, "file_id", "kind")
		if err == nil {
			result, err = h.Service.ImportJob(r.Context(), a, in, key, rid)
		}
	case r.URL.Path == "/api/v1/admin/export-jobs" && r.Method == "POST":
		var in operations.ExportInput
		err = operationBody(w, r, &in, "kind")
		if err == nil {
			result, err = h.Service.ExportJob(r.Context(), a, in, key, rid)
		}
	case strings.HasPrefix(r.URL.Path, "/api/v1/admin/import-jobs/") && strings.HasSuffix(r.URL.Path, "/commit") && r.Method == "POST":
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/import-jobs/"), "/commit")
		var in operations.VersionReason
		err = operationBody(w, r, &in, "version", "reason")
		if err == nil {
			result, err = h.Service.CommitImport(r.Context(), a, id, in, key, rid)
		}
	case strings.HasPrefix(r.URL.Path, "/api/v1/admin/jobs/") && r.Method == "GET":
		status = 200
		if len(r.URL.Query()) != 0 {
			err = &identity.Fault{Status: 400, Code: "INVALID_ARGUMENT"}
		} else {
			result, err = h.Service.GetJob(r.Context(), a, strings.TrimPrefix(r.URL.Path, "/api/v1/admin/jobs/"))
		}
	default:
		err = &identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"}
	}
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(result)
}

// servePrivateFiles 用途：处理加密 CSV 上传和受控报表下载，其他文件委托原处理器。
// 参数：w 响应器；r 文件请求；next 现有图片与凭证处理器。
// 返回值：无；原始导入文件不能下载，multipart 不落明文临时文件。
func (h OperationsHandler) servePrivateFiles(w http.ResponseWriter, r *http.Request, next http.Handler) {
	permission := "file.read"
	if r.URL.Path == "/api/v1/admin/files" {
		permission = "file.write"
	}
	a, err := h.operationActor(r, "ADMIN", permission)
	if err != nil {
		h.operationError(w, r, err)
		return
	}
	if r.Method != "POST" {
		h.operationError(w, r, &identity.Fault{Status: 405, Code: "METHOD_NOT_ALLOWED"})
		return
	}
	if r.URL.Path == "/api/v1/admin/files" {
		r.Body = http.MaxBytesReader(w, r.Body, 6*1024*1024)
		if err = r.ParseMultipartForm(6 * 1024 * 1024); err != nil {
			h.operationError(w, r, &identity.Fault{Status: 422, Code: "INVALID_ARGUMENT"})
			return
		}
		defer r.MultipartForm.RemoveAll()
		if len(r.MultipartForm.Value) != 1 || len(r.MultipartForm.Value["kind"]) != 1 || len(r.MultipartForm.File) != 1 || len(r.MultipartForm.File["file"]) != 1 {
			h.operationError(w, r, &identity.Fault{Status: 422, Code: "INVALID_ARGUMENT"})
			return
		}
		if r.FormValue("kind") == "IMAGE" {
			next.ServeHTTP(w, r)
			return
		}
		if r.FormValue("kind") != "IMPORT" {
			h.operationError(w, r, &identity.Fault{Status: 422, Code: "INVALID_ARGUMENT"})
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			h.operationError(w, r, &identity.Fault{Status: 422, Code: "INVALID_ARGUMENT"})
			return
		}
		defer file.Close()
		raw, err := io.ReadAll(io.LimitReader(file, 5*1024*1024+1))
		if err != nil {
			h.operationError(w, r, err)
			return
		}
		defer clear(raw)
		result, err := h.Service.UploadImport(r.Context(), a, raw, r.Header.Get("Idempotency-Key"), w.Header().Get("X-Request-ID"))
		if err != nil {
			h.operationError(w, r, err)
			return
		}
		w.WriteHeader(201)
		json.NewEncoder(w).Encode(result)
		return
	}
	id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/admin/files/"), "/download")
	if !rules.ValidID(id) {
		h.operationError(w, r, &identity.Fault{Status: 422, Code: "INVALID_ARGUMENT"})
		return
	}
	var kind string
	if err = h.Service.DB.QueryRowContext(r.Context(), "SELECT kind FROM files WHERE id=?", id).Scan(&kind); err != nil {
		if err == sql.ErrNoRows {
			err = &identity.Fault{Status: 404, Code: "NOT_FOUND"}
		}
		h.operationError(w, r, err)
		return
	}
	if kind == "CREDENTIAL_EXPORT" {
		next.ServeHTTP(w, r)
		return
	}
	started := false
	err = h.Service.DownloadReport(r.Context(), a, id, w.Header().Get("X-Request-ID"), func(reader io.Reader, name string) error {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
		w.Header().Set("Cache-Control", "no-store")
		started = true
		_, err := io.Copy(w, reader)
		return err
	})
	if err != nil && !started {
		h.operationError(w, r, err)
	}
}
