// Copyright 2015-2016 Apcera Inc. All rights reserved.

package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"

	"github.com/apcera/util/wsconn"
	"github.com/appc/spec/schema/types"
	"github.com/gorilla/rpc/v2/json2"
	"github.com/gorilla/websocket"
)

type Client interface {
	Info() (*HostInfo, error)

	CreatePod(req *PodCreateRequest) (*Pod, error)
	ListPods() ([]*Pod, error)
	GetPod(uuid string) (*Pod, error)
	DestroyPod(uuid string) error
	EnterContainer(uuid string, appName string, app *types.App) (net.Conn, error)

	CreateImage(reader io.Reader) (*Image, error)
	ListImages() ([]*Image, error)
	GetImage(hash string) (*Image, error)
	DeleteImage(hash string) error
}

type client struct {
	HttpClient *http.Client
	baseUrl    string
	conn       string
	dialer     func() (net.Conn, error)
}

func New(conn string) (Client, error) {
	c := &client{
		HttpClient: http.DefaultClient,
		conn:       conn,
	}
	u, err := url.Parse(conn)
	if err != nil {
		return nil, err
	}

	switch u.Scheme {
	case "unix":
		if u.Path == "" {
			u.Path = u.Host
		}
		tr := &http.Transport{
			Dial: func(proto, addr string) (conn net.Conn, err error) {
				return net.Dial("unix", u.Path)
			},
		}
		c.HttpClient = &http.Client{Transport: tr}
		c.baseUrl = "http://kurmaos"
		c.dialer = func() (net.Conn, error) { return net.Dial("unix", u.Path) }
	case "http", "https":
		c.baseUrl = u.String()
		c.dialer = func() (net.Conn, error) { return net.Dial("tcp", u.Host) }
	case "tcp":
		u.Scheme = "http"
		c.baseUrl = u.String()
		c.dialer = func() (net.Conn, error) { return net.Dial("tcp", u.Host) }
	default:
		return nil, fmt.Errorf("unrecognized protocol scheme %q specified", u.Scheme)
	}

	return c, nil
}

func (c *client) Info() (*HostInfo, error) {
	u, err := url.Parse(c.baseUrl)
	if err != nil {
		return nil, err
	}
	u.Path = "/info"

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("request failed with non-200 status: %s", resp.Status)
	}

	var hostInfo *HostInfo
	if err := json.NewDecoder(resp.Body).Decode(&hostInfo); err != nil {
		return nil, err
	}
	return hostInfo, nil
}

func (c *client) CreatePod(req *PodCreateRequest) (*Pod, error) {
	var resp *PodResponse
	err := c.execute("Pods.Create", req, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Pod, nil
}

func (c *client) ListPods() ([]*Pod, error) {
	var resp *PodListResponse
	err := c.execute("Pods.List", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Pods, nil
}

func (c *client) GetPod(uuid string) (*Pod, error) {
	var resp *PodResponse
	err := c.execute("Pods.Get", uuid, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Pod, nil
}

func (c *client) DestroyPod(uuid string) error {
	return c.execute("Pods.Destroy", uuid, nil)
}

func (c *client) EnterContainer(uuid string, appName string, app *types.App) (net.Conn, error) {
	u, err := url.Parse(c.baseUrl)
	if err != nil {
		return nil, err
	}
	u.Path = "/containers/enter"

	// set headers
	headers := http.Header{
		"Origin": {u.String()},
	}
	u.Scheme = "ws"

	// dial the connection
	conn, err := c.dialer()
	if err != nil {
		return nil, err
	}

	// initialize the websocket
	ws, _, err := websocket.NewClient(conn, u, headers, 1024, 1024)
	if err != nil {
		return nil, err
	}

	// build the runlist
	er := ContainerEnterRequest{UUID: uuid, AppName: appName, App: *app}
	if err := ws.WriteJSON(er); err != nil {
		return nil, err
	}

	// create the websocket connection
	wsc := wsconn.NewWebsocketConnection(ws)
	return wsc, nil
}

func (c *client) CreateImage(reader io.Reader) (*Image, error) {
	u, err := url.Parse(c.baseUrl)
	if err != nil {
		return nil, err
	}
	u.Path = "/images/create"

	req, err := http.NewRequest("POST", u.String(), reader)
	if err != nil {
		return nil, err
	}

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("request did not return a 201 Created status: %s", resp.Status)
	}

	var imageResp *ImageResponse
	if err := json.NewDecoder(resp.Body).Decode(&imageResp); err != nil {
		return nil, err
	}
	return imageResp.Image, nil
}

func (c *client) ListImages() ([]*Image, error) {
	var resp *ImageListResponse
	err := c.execute("Images.List", nil, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Images, nil
}

func (c *client) GetImage(hash string) (*Image, error) {
	var resp *ImageResponse
	err := c.execute("Images.Get", hash, &resp)
	if err != nil {
		return nil, err
	}
	return resp.Image, nil
}

func (c *client) DeleteImage(hash string) error {
	return c.execute("Images.Delete", hash, nil)
}

func (c *client) execute(cmd string, args, reply interface{}) error {
	buf, err := json2.EncodeClientRequest(cmd, args)
	if err != nil {
		return err
	}
	body := bytes.NewBuffer(buf)
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/rpc", c.baseUrl), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.HttpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if reply != nil {
		return json2.DecodeClientResponse(resp.Body, reply)
	} else {
		return nil
	}
}
