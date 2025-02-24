package debridlink

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/MunifTanjim/stremthru/core"
	"github.com/MunifTanjim/stremthru/store"
)

type StoreClient struct {
	Name   store.StoreName
	client *APIClient
}

func NewStoreClient() *StoreClient {
	c := &StoreClient{}
	c.client = NewAPIClient(&APIClientConfig{})
	c.Name = store.StoreNameDebridLink
	return c
}

func (c *StoreClient) GetName() store.StoreName {
	return c.Name
}

func (c *StoreClient) GetUser(params *store.GetUserParams) (*store.User, error) {
	res, err := c.client.GetAccountInfo(&GetAccountInfoParams{
		Ctx: params.Ctx,
	})
	if err != nil {
		return nil, err
	}
	url, err := url.Parse(res.Data.ViewSessidUrl)
	if err != nil {
		return nil, err
	}
	id := strings.Split(url.Path, "/")[2]
	data := &store.User{
		Id:    id,
		Email: res.Data.Email,
	}
	if res.Data.PremiumLeft != 0 {
		data.SubscriptionStatus = store.UserSubscriptionStatusPremium
	} else {
		data.SubscriptionStatus = store.UserSubscriptionStatusExpired
	}
	return data, err
}

func (c *StoreClient) CheckMagnet(params *store.CheckMagnetParams) (*store.CheckMagnetData, error) {
	magnetByLink := map[string]core.MagnetLink{}
	magnetLinks := []string{}

	for _, m := range params.Magnets {
		magnet, err := core.ParseMagnetLink(m)
		if err != nil {
			return nil, err
		}
		magnetLinks = append(magnetLinks, magnet.Link)
		magnetByLink[magnet.Link] = magnet
	}

	res, err := c.client.CheckSeedboxTorrentsCached(&CheckSeedboxTorrentsCachedParams{
		Ctx:  params.Ctx,
		Urls: magnetLinks,
	})
	if err != nil {
		return nil, err
	}

	data := &store.CheckMagnetData{}

	for _, mLink := range magnetLinks {
		item := &store.CheckMagnetDataItem{
			Hash:   magnetByLink[mLink].Hash,
			Magnet: magnetByLink[mLink].Link,
			Status: store.MagnetStatusUnknown,
			Files:  []store.MagnetFile{},
		}

		if result, ok := res.Data[mLink]; ok {
			for idx, f := range result.Files {
				item.Status = store.MagnetStatusCached

				item.Files = append(item.Files, store.MagnetFile{
					Idx:  idx,
					Name: f.Name,
					Size: f.Size,
				})
			}
		}

		data.Items = append(data.Items, *item)
	}

	return data, nil
}

func (c *StoreClient) AddMagnet(params *store.AddMagnetParams) (*store.AddMagnetData, error) {
	magnet, err := core.ParseMagnetLink(params.Magnet)
	if err != nil {
		return nil, err
	}
	res, err := c.client.AddSeedboxTorrent(&AddSeedboxTorrentParams{
		Ctx:           params.Ctx,
		Url:           magnet.Link,
		StructureType: SeedboxTorrentStructureTypeList,
	})
	if err != nil {
		return nil, err
	}

	t := res.Data

	data := &store.AddMagnetData{
		Id:     t.Id,
		Hash:   t.HashString,
		Magnet: magnet.Link,
		Name:   t.Name,
		Status: store.MagnetStatusQueued,
		Files:  []store.MagnetFile{},
	}

	if t.DownloadPercent == 100 {
		data.Status = store.MagnetStatusDownloaded

		for idx, f := range t.Files {
			file := &store.MagnetFile{
				Idx:  idx,
				Name: f.Name,
				Size: f.Size,
				Link: f.DownloadUrl,
			}

			data.Files = append(data.Files, *file)
		}
	}

	return data, nil
}

func (c *StoreClient) GetMagnet(params *store.GetMagnetParams) (*store.GetMagnetData, error) {
	res, err := c.client.ListSeedboxTorrents(&ListSeedboxTorrentsParams{
		Ctx: params.Ctx,
		Ids: []string{params.Id},
	})
	if err != nil {
		return nil, err
	}
	if len(res.Data.Value) != 1 || res.Data.Value[0].Id != params.Id {
		err := core.NewAPIError("not found")
		err.StatusCode = http.StatusNotFound
		err.StoreName = string(store.StoreNameDebridLink)
		return nil, err
	}
	t := res.Data.Value[0]
	data := &store.GetMagnetData{
		Id:     t.Id,
		Hash:   t.HashString,
		Name:   t.Name,
		Status: store.MagnetStatusUnknown,
		Files:  []store.MagnetFile{},
	}
	if t.DownloadPercent == 100 {
		data.Status = store.MagnetStatusDownloaded
	} else if t.DownloadPercent < 100 {
		data.Status = store.MagnetStatusDownloading
	}
	for idx, f := range t.Files {
		file := &store.MagnetFile{
			Idx:  idx,
			Link: f.DownloadUrl,
			Name: f.Name,
			Path: "",
			Size: f.Size,
		}

		data.Files = append(data.Files, *file)
	}
	return data, nil
}

func (c *StoreClient) ListMagnets(params *store.ListMagnetsParams) (*store.ListMagnetsData, error) {
	res, err := c.client.ListSeedboxTorrents(&ListSeedboxTorrentsParams{
		Ctx: params.Ctx,
	})
	if err != nil {
		return nil, err
	}
	data := &store.ListMagnetsData{}
	for _, t := range res.Data.Value {
		item := &store.ListMagnetsDataItem{
			Id:     t.Id,
			Hash:   t.HashString,
			Name:   t.Name,
			Status: store.MagnetStatusUnknown,
		}
		if t.DownloadPercent == 100 {
			item.Status = store.MagnetStatusDownloaded
		} else if t.DownloadPercent < 100 {
			item.Status = store.MagnetStatusDownloading
		}
		data.Items = append(data.Items, *item)
	}
	return data, nil
}

func (c *StoreClient) RemoveMagnet(params *store.RemoveMagnetParams) (*store.RemoveMagnetData, error) {
	res, err := c.client.RemoveSeedboxTorrents(&RemoveSeedboxTorrentParams{
		Ctx: params.Ctx,
		Ids: []string{params.Id},
	})
	if err != nil {
		return nil, err
	}
	data := &store.RemoveMagnetData{
		Id: res.Data[0],
	}
	return data, nil
}

func (c *StoreClient) GenerateLink(params *store.GenerateLinkParams) (*store.GenerateLinkData, error) {
	data := &store.GenerateLinkData{Link: params.Link}
	return data, nil
}
