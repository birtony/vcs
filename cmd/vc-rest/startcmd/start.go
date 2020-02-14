/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package startcmd

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	ariesapi "github.com/hyperledger/aries-framework-go/pkg/framework/aries/api"
	vdriapi "github.com/hyperledger/aries-framework-go/pkg/framework/aries/api/vdri"
	"github.com/hyperledger/aries-framework-go/pkg/framework/context"
	"github.com/hyperledger/aries-framework-go/pkg/kms/legacykms"
	"github.com/hyperledger/aries-framework-go/pkg/storage"
	"github.com/hyperledger/aries-framework-go/pkg/storage/mem"
	vdripkg "github.com/hyperledger/aries-framework-go/pkg/vdri"
	"github.com/hyperledger/aries-framework-go/pkg/vdri/httpbinding"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/trustbloc/edge-core/pkg/storage/memstore"
	"github.com/trustbloc/edv/pkg/client/edv"

	"github.com/trustbloc/edge-service/pkg/restapi/vc"
	cmdutils "github.com/trustbloc/edge-service/pkg/utils/cmd"
)

const (
	hostURLFlagName                   = "host-url"
	hostURLFlagShorthand              = "u"
	hostURLFlagUsage                  = "URL to run the vc-rest instance on. Format: HostName:Port."
	hostURLEnvKey                     = "VC_REST_HOST_URL"
	edvURLFlagName                    = "edv-url"
	edvURLFlagShorthand               = "e"
	edvURLFlagUsage                   = "URL EDV instance is running on. Format: HostName:Port."
	edvURLEnvKey                      = "EDV_REST_HOST_URL"
	sideTreeURLFlagName               = "sidetree-url"
	sideTreeURLFlagUsage              = "URL SideTree instance is running on. Format: HostName:Port."
	sideTreeURLEnvKey                 = "SIDETREE_HOST_URL"
	hostURLExternalFlagName           = "host-url-external"
	hostURLExternalEnvKey             = "VC_REST_HOST_URL_EXTERNAL"
	agentInboundHostExternalFlagUsage = "Host External Name:Port This is the URL for the host server as seen externally." +
		" If not provided, then the host url will be used here." +
		" Alternatively, this can be set with the following environment variable: " + hostURLExternalEnvKey
)

type vcRestParameters struct {
	srv             server
	hostURL         string
	edvURL          string
	sideTreeURL     string
	hostURLExternal string
}

type server interface {
	ListenAndServe(host string, router http.Handler) error
}

// HTTPServer represents an actual HTTP server implementation.
type HTTPServer struct{}

// ListenAndServe starts the server using the standard Go HTTP server implementation.
func (s *HTTPServer) ListenAndServe(host string, router http.Handler) error {
	return http.ListenAndServe(host, router)
}

// GetStartCmd returns the Cobra start command.
func GetStartCmd(srv server) *cobra.Command {
	startCmd := createStartCmd(srv)

	createFlags(startCmd)

	return startCmd
}

func createStartCmd(srv server) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start vc-rest",
		Long:  "Start vc-rest inside the edge-service",
		RunE: func(cmd *cobra.Command, args []string) error {
			hostURL, err := cmdutils.GetUserSetVar(cmd, hostURLFlagName, hostURLEnvKey, false)
			if err != nil {
				return err
			}

			edvURL, err := cmdutils.GetUserSetVar(cmd, edvURLFlagName, edvURLEnvKey, false)
			if err != nil {
				return err
			}

			sideTreeURL, err := cmdutils.GetUserSetVar(cmd, sideTreeURLFlagName, sideTreeURLEnvKey, false)
			if err != nil {
				return err
			}

			hostURLExternal, err := cmdutils.GetUserSetVar(cmd, hostURLExternalFlagName,
				hostURLExternalEnvKey, true)
			if err != nil {
				return err
			}

			parameters := &vcRestParameters{
				srv:             srv,
				hostURL:         hostURL,
				edvURL:          edvURL,
				sideTreeURL:     sideTreeURL,
				hostURLExternal: hostURLExternal,
			}
			return startEdgeService(parameters)
		},
	}
}

func createFlags(startCmd *cobra.Command) {
	startCmd.Flags().StringP(hostURLFlagName, hostURLFlagShorthand, "", hostURLFlagUsage)
	startCmd.Flags().StringP(edvURLFlagName, edvURLFlagShorthand, "", edvURLFlagUsage)
	startCmd.Flags().StringP(sideTreeURLFlagName, "", "", sideTreeURLFlagUsage)
	startCmd.Flags().StringP(hostURLExternalFlagName, "", "", agentInboundHostExternalFlagUsage)
}

func startEdgeService(parameters *vcRestParameters) error {
	// Create KMS
	kms, err := createKMS(mem.NewProvider())
	if err != nil {
		return err
	}

	// Create VDRI
	vdri, err := createVDRI(parameters.sideTreeURL, kms)
	if err != nil {
		return err
	}

	externalHostURL := parameters.hostURL
	if parameters.hostURLExternal != "" {
		externalHostURL = parameters.hostURLExternal
	}

	vcService, err := vc.New(memstore.NewProvider(), edv.New(parameters.edvURL), kms, vdri, externalHostURL)
	if err != nil {
		return err
	}

	handlers := vcService.GetOperations()
	router := mux.NewRouter()

	for _, handler := range handlers {
		router.HandleFunc(handler.Path(), handler.Handle()).Methods(handler.Method())
	}

	log.Infof("Starting vc rest server on host %s", parameters.hostURL)

	return parameters.srv.ListenAndServe(parameters.hostURL, router)
}

func createKMS(s storage.Provider) (ariesapi.CloseableKMS, error) {
	kmsProvider, err := context.New(context.WithStorageProvider(s))
	if err != nil {
		return nil, fmt.Errorf("failed to create new kms provider: %w", err)
	}

	kms, err := legacykms.New(kmsProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create new kms: %w", err)
	}

	return kms, nil
}

func createVDRI(sideTreeURL string, kms legacykms.KMS) (vdriapi.Registry, error) {
	sideTreeVDRI, err := httpbinding.New(sideTreeURL,
		httpbinding.WithAccept(func(method string) bool { return method == "sidetree" }))
	if err != nil {
		return nil, fmt.Errorf("failed to create new sidetree vdri: %w", err)
	}

	vdriProvider, err := context.New(context.WithLegacyKMS(kms))
	if err != nil {
		return nil, fmt.Errorf("failed to create new vdri provider: %w", err)
	}

	return vdripkg.New(vdriProvider, vdripkg.WithVDRI(sideTreeVDRI)), nil
}
