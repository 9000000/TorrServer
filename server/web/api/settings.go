package api

import (
	"net/http"
	"net/url"

	"server/rutor"

	"server/dlna"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"

	sets "server/settings"
	"server/torr"
	"server/torr/utils"
)

// Action: get, set, def
type setsReqJS struct {
	requestI
	Sets *sets.BTSets `json:"sets,omitempty"`
}

// settings godoc
//
//	@Summary		Get / Set server settings
//	@Description	Allow to get or set server settings.
//
//	@Tags			API
//
//	@Param			request	body	setsReqJS	true	"Settings request. Available params for action: get, set, def"
//
//	@Accept			json
//	@Produce		json
//	@Success		200	{object}	sets.BTSets	"Settings JSON or nothing. Depends on what action has been asked."
//	@Router			/settings [post]
func settings(c *gin.Context) {
	var req setsReqJS
	err := c.ShouldBindJSON(&req)
	if err != nil {
		c.AbortWithError(http.StatusBadRequest, err)
		return
	}

	if req.Action == "get" {
		c.JSON(200, sets.BTsets)
		return
	} else if req.Action == "set" {
		torr.SetSettings(req.Sets)
		dlna.Stop()
		if req.Sets.EnableDLNA {
			dlna.Start()
		}
		rutor.Stop()
		rutor.Start()
		c.Status(200)
		return
	} else if req.Action == "def" {
		torr.SetDefSettings()
		dlna.Stop()
		rutor.Stop()
		c.Status(200)
		return
	} else if req.Action == "refresh_proxy" {
		if sets.BTsets.ProxyListURL == "" {
			c.AbortWithError(http.StatusBadRequest, errors.New("ProxyListURL is empty"))
			return
		}
		newProxy, err := utils.FetchRandomProxy(sets.BTsets.ProxyListURL, sets.BTsets.ProxyTypeFilter)
		if err != nil {
			c.AbortWithError(http.StatusInternalServerError, errors.Wrap(err, "Failed to fetch proxy"))
			return
		}
		if newProxy == "" {
			c.AbortWithError(http.StatusNotFound, errors.New("No valid proxies found in list"))
			return
		}
		
		sets.BTsets.BitTorrentProxyURL = newProxy
		
		// Restart torrent client with new proxy
		torr.SetSettings(sets.BTsets)
		dlna.Stop()
		if sets.BTsets.EnableDLNA {
			dlna.Start()
		}
		rutor.Stop()
		rutor.Start()

		// Return settings with masked proxy URL for security
		masked := maskProxyURL(newProxy)
		c.JSON(200, gin.H{"BitTorrentProxyURL": masked})
		return
	}
	c.AbortWithError(http.StatusBadRequest, errors.New("action is empty"))
}

// maskProxyURL hides credentials from a proxy URL, keeping only scheme://host:port
func maskProxyURL(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	// Rebuild without userinfo
	masked := parsed.Scheme + "://" + parsed.Host
	return masked
}
