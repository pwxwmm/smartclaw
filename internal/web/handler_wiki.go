package web

import (
	"context"
	"encoding/json"
)

func (h *Handler) handleWikiSearchWS(client *Client, msg WSMessage) {
	if h.wikiClient == nil || !h.wikiClient.IsEnabled() {
		h.sendToClient(client, WSResponse{Type: "wiki_search", Data: map[string]any{
			"enabled": false,
			"results": []any{},
		}})
		return
	}
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid wiki search request")
		return
	}
	query, _ := data["query"].(string)
	limit := 5
	if l, ok := data["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}
	result, err := h.wikiClient.Search(context.Background(), query, limit)
	if err != nil {
		h.sendError(client, "Wiki search failed: "+err.Error())
		return
	}
	h.sendToClient(client, WSResponse{Type: "wiki_search", Data: result})
}

func (h *Handler) handleWikiPagesWS(client *Client) {
	if h.wikiClient == nil || !h.wikiClient.IsEnabled() {
		h.sendToClient(client, WSResponse{Type: "wiki_pages", Data: map[string]any{
			"enabled": false,
			"pages":   []any{},
		}})
		return
	}
	pages, err := h.wikiClient.ListPages(context.Background(), 50)
	if err != nil {
		h.sendError(client, "Wiki list failed: "+err.Error())
		return
	}
	h.sendToClient(client, WSResponse{Type: "wiki_pages", Data: map[string]any{
		"enabled": true,
		"pages":   pages,
	}})
}

func (h *Handler) handleWikiPageContentWS(client *Client, msg WSMessage) {
	if h.wikiClient == nil || !h.wikiClient.IsEnabled() {
		h.sendToClient(client, WSResponse{Type: "wiki_page_content", Data: map[string]any{
			"enabled": false,
		}})
		return
	}
	var data map[string]any
	if err := json.Unmarshal(msg.Data, &data); err != nil {
		h.sendError(client, "Invalid wiki page content request")
		return
	}
	pageID, _ := data["page_id"].(string)
	if pageID == "" {
		h.sendError(client, "page_id is required")
		return
	}
	page, err := h.wikiClient.GetPage(context.Background(), pageID)
	if err != nil {
		h.sendError(client, "Wiki get page failed: "+err.Error())
		return
	}
	h.sendToClient(client, WSResponse{Type: "wiki_page_content", Data: map[string]any{
		"enabled": true,
		"page":    page,
	}})
}
