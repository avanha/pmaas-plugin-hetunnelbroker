package http

import (
	"embed"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"sort"

	"github.com/avanha/pmaas-plugin-hetunnelbroker/data"
	"github.com/avanha/pmaas-plugin-hetunnelbroker/internal/common"
	"github.com/avanha/pmaas-spi"
)

//go:embed content/static content/templates
var contentFS embed.FS

var tunnelTemplate = spi.TemplateInfo{
	Name:   "he_tunnel",
	Paths:  []string{"templates/tunnel.htmlt"},
	Styles: []string{"css/tunnel.css"},
}

var statusTemplate = spi.TemplateInfo{
	Name:   "hetunnelbroker_status",
	Paths:  []string{"templates/hetunnelbroker_status.htmlt"},
	Styles: []string{"css/hetunnelbroker_status.css"},
}

type Handler struct {
	container   spi.IPMAASContainer
	entityStore common.EntityStore
}

func NewHandler() *Handler {
	return &Handler{}
}

func (h *Handler) Init(container spi.IPMAASContainer, entityStore common.EntityStore) {
	h.container = container
	h.entityStore = entityStore
	container.ProvideContentFS(&contentFS, "content")
	container.EnableStaticContent("static")
	container.AddRoute("/plugins/hetunnelbroker/", h.handleHttpListRequest)
	container.RegisterEntityRenderer(
		reflect.TypeOf((*data.PluginStatus)(nil)).Elem(),
		h.statusDataRendererFactory)
	container.RegisterEntityRenderer(
		reflect.TypeOf((*data.TunnelData)(nil)).Elem(),
		h.tunnelDataRendererFactory)
}

func (h *Handler) handleHttpListRequest(writer http.ResponseWriter, request *http.Request) {
	result, err := h.entityStore.GetStatusAndEntities()

	if err != nil {
		fmt.Printf("hetunnelbroker.http handleHttpListRequest: Error retrieving entities: %s\n", err)
		result = common.StatusAndEntities{}
	}

	sort.SliceStable(result.Tunnels, func(i, j int) bool {
		return result.Tunnels[i].Name < result.Tunnels[j].Name
	})

	// Convert the slice of structs to a slice of any
	entityListSize := len(result.Tunnels)
	entityPointers := make([]any, entityListSize)

	for i := 0; i < entityListSize; i++ {
		entityPointers[i] = &result.Tunnels[i]
	}

	h.container.RenderList(
		writer,
		request,
		spi.RenderListOptions{
			Title:  "HE Tunnelbroker",
			Header: &result.Status,
		},
		entityPointers)
}

func (h *Handler) statusDataRendererFactory() (spi.EntityRenderer, error) {
	// Load the template
	template, err := h.container.GetTemplate(&statusTemplate)

	if err != nil {
		return spi.EntityRenderer{}, fmt.Errorf("unable to load hetunnelbroker_status template: %v", err)
	}

	// Declare a function that casts the entity to the expected type and evaluates it via the template loaded above
	renderer := func(w io.Writer, entity any) error {
		status, ok := entity.(*data.PluginStatus)

		if !ok {
			return errors.New("item is not an instance of *PluginStatus")
		}

		err := template.Instance.Execute(w, status)

		if err != nil {
			return fmt.Errorf("unable to execute hetunnelbroker_status template: %w", err)
		}

		return nil
	}

	return spi.EntityRenderer{
		StreamingRenderFunc: renderer,
		Styles:              template.Styles,
		Scripts:             template.Scripts,
	}, nil
}

func (h *Handler) tunnelDataRendererFactory() (spi.EntityRenderer, error) {
	// Load the template
	template, err := h.container.GetTemplate(&tunnelTemplate)

	if err != nil {
		return spi.EntityRenderer{}, fmt.Errorf("unable to load tunnel template: %v", err)
	}

	// Declare a function that casts the entity to the expected type and evaluates it via the template loaded above
	renderer := func(w io.Writer, entity any) error {
		tunnelData, ok := entity.(*data.TunnelData)

		if !ok {
			return errors.New("item is not an instance of *TunnelData")
		}

		err := template.Instance.Execute(w, tunnelData)

		if err != nil {
			return fmt.Errorf("unable to execute tunnel template: %w", err)
		}

		return nil
	}

	return spi.EntityRenderer{
		StreamingRenderFunc: renderer,
		Styles:              template.Styles,
		Scripts:             template.Scripts,
	}, nil
}
