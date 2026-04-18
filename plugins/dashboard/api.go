package dashboard

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// getState 处理获取比赛状态总览的请求。
// 返回比赛信息、题目列表（含解题数和一血）、排行榜、统计数据和最近事件。
func (p *Plugin) getState(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	// 验证比赛 ID 格式（32 字符 UUID）
	if len(compID) != 32 {
		http.Error(w, `{"error":"invalid competition id"}`, http.StatusBadRequest)
		return
	}

	// 获取完整的比赛状态数据
	state, err := p.getCompetitionState(r.Context(), compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(state)
}

// getFirstBlood 处理获取比赛一血列表的请求。
// 返回指定比赛中每道题目的首个正确提交者信息。
func (p *Plugin) getFirstBlood(w http.ResponseWriter, r *http.Request) {
	compID := chi.URLParam(r, "id")
	if len(compID) != 32 {
		http.Error(w, `{"error":"invalid competition id"}`, http.StatusBadRequest)
		return
	}

	// 查询一血记录
	list, err := p.getFirstBloodList(r.Context(), compID)
	if err != nil {
		http.Error(w, `{"error":"internal"}`, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(list)
}
