package web

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	g "github.com/maragudk/gomponents"
	c "github.com/maragudk/gomponents/components"
	. "github.com/maragudk/gomponents/html"
	"github.com/tgragnato/magnetico/persistence"
)

func torrents() g.Node {
	return c.HTML5(c.HTML5Props{
		Title:       "Search - magnetico",
		Description: "A self-hosted BitTorrent DHT search engine",
		Language:    "en",
		Head: []g.Node{
			Meta(Charset("utf-8")),
			Meta(Name("viewport"), Content("width=device-width, initial-scale=1")),
			Link(Rel("stylesheet"), Href("/static/styles/reset.css")),
			Link(Rel("stylesheet"), Href("/static/styles/essential.css")),
			Link(Rel("stylesheet"), Href("/static/styles/torrents.css")),
			Script(Src("/static/scripts/mustache-v2.3.0.min.js")),
			Script(Src("/static/scripts/common.js")),
			Script(Src("/static/scripts/torrents.js")),
			Script(
				ID("item-template"),
				Type("text/x-handlebars-template"),
				Li(
					Div(
						H3(A(Href("/torrents/{{ infoHash }}"), g.Text("{{ name }}"))),
						A(
							Href("magnet:?xt=urn:btih:{{ infoHash }}&dn={{ name }}"),
							Img(
								Src("/static/assets/magnet.gif"),
								Alt("Magnet link")),
							Title("Download this torrent using magnet"),
						),
						Small(g.Text("{{ infoHash }}")),
					),
					g.Text("{{ size }}, {{ discoveredOn }}"),
				),
			),
		},
		Body: []g.Node{
			Header(
				Div(
					A(Href("/"),
						B(g.Text("magnetico"))),
				),
				Form(
					Action("/torrents"),
					Method("get"),
					AutoComplete("off"),
					Role("search"),
					Input(
						Type("search"),
						Name("query"),
						Placeholder("Search the BitTorrent DHT"),
					),
				),
				Div(
					A(
						Href("/feed"),
						ID("feed-anchor"),
						Img(
							Src("/static/assets/feed.png"),
							Alt("RSS feed icon"),
							Title("subscribe to the RSS feed"),
						),
						g.Text("subscribe"),
					),
				),
			),
			Main(Ul()),
			Footer(
				Button(
					g.Attr("onclick", "load();"),
					g.Text("Load More Results"),
				),
			),
		},
	})
}

func torrentsHandler(w http.ResponseWriter, r *http.Request) {
	if err := torrents().Render(w); err != nil {
		http.Error(w, "Torrents render "+err.Error(), http.StatusInternalServerError)
	}
}

func apiTorrents(w http.ResponseWriter, r *http.Request) {
	// @lastOrderedValue AND @lastID are either both supplied or neither of them should be supplied
	// at all; and if that is NOT the case, then return an error.
	if q := r.URL.Query(); !((q.Get("lastOrderedValue") != "" && q.Get("lastID") != "") ||
		(q.Get("lastOrderedValue") == "" && q.Get("lastID") == "")) {
		http.Error(w, "`lastOrderedValue`, `lastID` must be supplied altogether, if supplied.", http.StatusBadRequest)
		return
	}

	var tq struct {
		Epoch            int64    `schema:"epoch"`
		Query            string   `schema:"query"`
		OrderBy          string   `schema:"orderBy"`
		Ascending        bool     `schema:"ascending"`
		LastOrderedValue *float64 `schema:"lastOrderedValue"`
		LastID           *uint64  `schema:"lastID"`
		Limit            uint64   `schema:"limit"`
	}

	err := r.ParseForm()
	if err != nil {
		http.Error(w, "error while parsing the URL: "+err.Error(), http.StatusBadRequest)
		return
	}

	if r.Form.Has("epoch") {
		tq.Epoch, err = strconv.ParseInt(r.Form.Get("epoch"), 10, 64)
		if err != nil {
			http.Error(w, "error while parsing the URL: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		tq.Epoch = time.Now().Unix()
	}

	tq.Query = r.Form.Get("query")
	tq.OrderBy = r.Form.Get("orderBy")

	var orderBy persistence.OrderingCriteria
	if tq.OrderBy == "" {
		if tq.Query == "" {
			orderBy = persistence.ByDiscoveredOn
		} else {
			orderBy = persistence.ByRelevance
		}
	} else {
		var err error
		orderBy, err = parseOrderBy(tq.OrderBy)
		if err != nil {
			http.Error(w, "error while parsing the URL: "+err.Error(), http.StatusBadRequest)
		}
	}

	if r.Form.Has("ascending") {
		tq.Ascending, err = strconv.ParseBool(r.Form.Get("ascending"))
		if err != nil {
			http.Error(w, "error while parsing the URL: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		tq.Ascending = true
	}

	if r.Form.Has("lastOrderedValue") {
		lastOrderedValue, err := strconv.ParseFloat(r.Form.Get("lastOrderedValue"), 64)
		if err != nil {
			http.Error(w, "error while parsing the URL: "+err.Error(), http.StatusBadRequest)
			return
		}
		tq.LastOrderedValue = &lastOrderedValue
	}

	if r.Form.Has("lastID") {
		lastID, err := strconv.ParseUint(r.Form.Get("lastID"), 10, 64)
		if err != nil {
			http.Error(w, "error while parsing the URL: "+err.Error(), http.StatusBadRequest)
			return
		}
		tq.LastID = &lastID
	}

	if r.Form.Has("limit") {
		tq.Limit, err = strconv.ParseUint(r.Form.Get("limit"), 10, 64)
		if err != nil {
			http.Error(w, "error while parsing the URL: "+err.Error(), http.StatusBadRequest)
			return
		}
	} else {
		tq.Limit = 20
	}

	torrents, err := database.QueryTorrents(
		tq.Query, tq.Epoch, orderBy,
		tq.Ascending, tq.Limit, tq.LastOrderedValue, tq.LastID)
	if err != nil {
		http.Error(w, "QueryTorrents: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set(ContentType, ContentTypeJson)
	if err = json.NewEncoder(w).Encode(torrents); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}

func parseOrderBy(s string) (persistence.OrderingCriteria, error) {
	switch s {
	case "RELEVANCE":
		return persistence.ByRelevance, nil

	case "TOTAL_SIZE":
		return persistence.ByTotalSize, nil

	case "DISCOVERED_ON":
		return persistence.ByDiscoveredOn, nil

	case "N_FILES":
		return persistence.ByNFiles, nil

	case "UPDATED_ON":
		return persistence.ByUpdatedOn, nil

	case "N_SEEDERS":
		return persistence.ByNSeeders, nil

	case "N_LEECHERS":
		return persistence.ByNLeechers, nil

	default:
		return persistence.ByDiscoveredOn, fmt.Errorf("unknown orderBy string: %s", s)
	}
}