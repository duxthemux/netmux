package api

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/duxthemux/netmux/app/nx-daemon/daemon"
	"github.com/duxthemux/netmux/business/caroot"
)

type API struct {
	Service *daemon.Daemon
}

func (a *API) pluginContext(ctx context.Context, router *mux.Router) {
	router.Name("contextConnect").
		Methods(http.MethodGet).
		Path("/{context}/connect").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			ctxName := mux.Vars(request)["context"]
			if ctxName == "" {
				http.Error(responseWriter, "context name is required", http.StatusBadRequest)

				return
			}

			err := a.Service.Connect(ctx, ctxName)
			if err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
			}
		})

	router.Name("contextDisconnect").
		Methods(http.MethodGet).
		Path("/{context}/disconnect").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			ctxName := mux.Vars(request)["context"]
			if ctxName == "" {
				http.Error(responseWriter, "context name is required", http.StatusBadRequest)

				return
			}

			err := a.Service.Disconnect(ctxName)
			if err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
			}
		})
}

func (a *API) pluginServices(ctx context.Context, router *mux.Router) {
	router.Name("servicesList").
		Methods(http.MethodGet).
		Path("/").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			svcs := a.Service.GetStatus()
			responseWriter.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(responseWriter).Encode(svcs)
			if err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
			}
		})

	router.Name("servicesStart").
		Methods(http.MethodGet).
		Path("/{context}/{name}/start").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			vars := mux.Vars(request)
			ctxName := vars["context"]
			svcName := vars["name"]
			if ctxName == "" {
				http.Error(responseWriter, "context name is required", http.StatusBadRequest)

				return
			}

			err := a.Service.StartService(ctx, ctxName, svcName)
			if err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
			}
		})

	router.Name("servicesStop").
		Methods(http.MethodGet).
		Path("/{context}/{name}/stop").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			vars := mux.Vars(request)
			ctxName := vars["context"]
			svcName := vars["name"]
			if ctxName == "" {
				http.Error(responseWriter, "context name is required", http.StatusBadRequest)

				return
			}

			err := a.Service.StopService(ctxName, svcName)
			if err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
			}
		})
}

func (a *API) pluginConfig(_ context.Context, router *mux.Router, caRoot *caroot.CA) {
	router.Name("configMain").
		Methods(http.MethodGet).
		Path("/main").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			cfg := a.Service.GetConfig()
			responseWriter.Header().Set("Content-Type", "application/json")
			err := json.NewEncoder(responseWriter).Encode(cfg)
			if err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
			}
		})

	router.Name("configHosts").
		Methods(http.MethodGet).
		Path("/hosts").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			cfg := a.Service.DNSEntries()
			responseWriter.Header().Set("Content-Type", "application/json")

			err := json.NewEncoder(responseWriter).Encode(cfg)
			if err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
			}
		})

	router.Name("configCa").
		Methods(http.MethodGet).
		Path("/caRoot").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			bytes, err := caRoot.CaCerBytes()
			if err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)

				return
			}
			responseWriter.Header().Set("Content-Type", "application/pkix-cert")

			_, _ = responseWriter.Write(bytes)
		})
}

func (a *API) pluginMisc(_ context.Context, router *mux.Router) {
	router.Name("miscReload").
		Methods(http.MethodGet).
		Path("/reload").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if err := a.Service.Reload(); err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
			}
		})

	router.Name("miscExit").
		Methods(http.MethodGet).
		Path("/exit").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			a.Service.Exit()
		})

	router.Name("miscCleanup").
		Methods(http.MethodGet).
		Path("/cleanup").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if err := a.Service.CleanUp(); err != nil {
				http.Error(responseWriter, err.Error(), http.StatusInternalServerError)
			}
		})
	router.Name("miscTest").
		Methods(http.MethodGet).
		Path("/test").
		HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			_, _ = responseWriter.Write([]byte("Netmux is here - OK!"))
		})
}

func (a *API) Plugin(ctx context.Context, rootRouter *mux.Router, ca *caroot.CA) error {
	apiV1Router := rootRouter.Name("apiV1-router").PathPrefix("/api/v1/").Subrouter()

	configRouter := apiV1Router.Name("userconfig-router").PathPrefix("/userconfig/").Subrouter()
	a.pluginConfig(ctx, configRouter, ca)

	ctxRouter := apiV1Router.Name("context-router").PathPrefix("/context/").Subrouter()
	a.pluginContext(ctx, ctxRouter)

	servicesRouter := apiV1Router.Name("services-router").PathPrefix("/services/").Subrouter()
	a.pluginServices(ctx, servicesRouter)

	miscRouter := apiV1Router.Name("misc-router").PathPrefix("/misc/").Subrouter()

	a.pluginMisc(ctx, miscRouter)

	return nil
}
