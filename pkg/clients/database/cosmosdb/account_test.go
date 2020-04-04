/*
Copyright 2020 The Crossplane Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cosmosdb

import (
	"testing"

	"github.com/Azure/azure-sdk-for-go/services/cosmos-db/mgmt/2015-04-08/documentdb"
	"github.com/google/go-cmp/cmp"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"

	"github.com/crossplane/crossplane-runtime/pkg/test"

	"github.com/crossplane/provider-azure/apis/database/v1beta1"
	azure "github.com/crossplane/provider-azure/pkg/clients"
)

func TestNewCosmosDBAccountClient(t *testing.T) {
	tests := []struct {
		name    string
		args    []byte
		wantRes *documentdb.DatabaseAccountsClient
		wantErr error
	}{
		{
			name:    "EmptyData",
			args:    []byte{},
			wantRes: nil,
			wantErr: errors.WithStack(errors.New("cannot unmarshal Azure client secret data: unexpected end of JSON input")),
		},
		{
			name: "Success",
			args: []byte(`{"clientId": "0f32e96b-b9a4-49ce-a857-243a33b20e5c",
	"clientSecret": "49d8cab5-d47a-4d1a-9133-5c5db29c345d",
	"subscriptionId": "bf1b0e59-93da-42e0-82c6-5a1d94227911",
	"tenantId": "302de427-dba9-4452-8583-a4268e46de6b",
	"activeDirectoryEndpointUrl": "https://login.microsoftonline.com",
	"resourceManagerEndpointUrl": "https://management.azure.com/",
	"activeDirectoryGraphResourceId": "https://graph.windows.net/",
	"sqlManagementEndpointUrl": "https://management.core.windows.net:8443/",
	"galleryEndpointUrl": "https://gallery.azure.com/",
	"managementEndpointUrl": "https://management.core.windows.net/"}`),
			wantRes: &documentdb.DatabaseAccountsClient{},
			wantErr: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewDatabaseAccountClient(tt.args)
			if diff := cmp.Diff(err, tt.wantErr, test.EquateErrors()); diff != "" {
				t.Errorf("NewCosmosDBAccountClient() error = %v, wantErr %v\n%s", err, tt.wantErr, diff)
			}
			if err != nil && got != nil {
				t.Errorf("NewCosmosDBAccountClient() %v, want nil", got)
			}
			if err == nil && got == nil {
				t.Errorf("NewCosmosDBAccountClient() %v, want not nil", got)
			}
		})
	}
}

func TestToDatabaseAccountCreate(t *testing.T) {
	resourceGroupName := "myrg"
	kind := documentdb.DatabaseAccountKind("MongoDB")
	location := "uswest"
	consistency := documentdb.DefaultConsistencyLevel("Eventual")

	t.Run("Nil", func(t *testing.T) {
		assert.Equal(t, documentdb.DatabaseAccountCreateUpdateParameters{}, ToDatabaseAccountCreate(nil))
	})
	t.Run("HappyCase", func(t *testing.T) {
		assert.Equal(t, documentdb.DatabaseAccountCreateUpdateParameters{
			Kind:     kind,
			Location: &location,
			DatabaseAccountCreateUpdateProperties: &documentdb.DatabaseAccountCreateUpdateProperties{
				ConsistencyPolicy: &documentdb.ConsistencyPolicy{
					DefaultConsistencyLevel: consistency,
					MaxIntervalInSeconds:    azure.ToInt32Ptr(10),
				},
				Locations: &[]documentdb.Location{
					{
						LocationName:     &location,
						FailoverPriority: azure.ToInt32Ptr(0, azure.FieldRequired),
						IsZoneRedundant:  azure.ToBoolPtr(true),
					},
				},
			},
		}, ToDatabaseAccountCreate(&v1beta1.CosmosDBAccountSpec{
			ForProvider: v1beta1.CosmosDBAccountParameters{
				ResourceGroupName: resourceGroupName,
				Kind:              kind,
				Location:          location,
				Properties: v1beta1.CosmosDBAccountProperties{
					ConsistencyPolicy: &v1beta1.CosmosDBAccountConsistencyPolicy{
						DefaultConsistencyLevel: "Eventual",
						MaxIntervalInSeconds:    azure.ToInt32Ptr(10),
					},
					Locations: []v1beta1.CosmosDBAccountLocation{
						{
							LocationName:     location,
							FailoverPriority: 0,
							IsZoneRedundant:  true,
						},
					},
				},
			},
		}))
	})
}

func TestCheckEqualDatabaseProperties(t *testing.T) {
	location := "uswest"

	t.Run("NotEqualLocation", func(t *testing.T) {
		assert.False(t, CheckEqualDatabaseProperties(
			v1beta1.CosmosDBAccountProperties{
				Locations: []v1beta1.CosmosDBAccountLocation{
					{
						LocationName: location,
					},
				},
			},
			v1beta1.CosmosDBAccountProperties{
				Locations: []v1beta1.CosmosDBAccountLocation{
					{
						LocationName: "some other location",
					},
				},
			}))
	})
	t.Run("EqualLocation", func(t *testing.T) {
		assert.False(t, CheckEqualDatabaseProperties(
			v1beta1.CosmosDBAccountProperties{
				Locations: []v1beta1.CosmosDBAccountLocation{
					{
						LocationName:     location,
						FailoverPriority: 1,
					},
				},
			},
			v1beta1.CosmosDBAccountProperties{
				Locations: []v1beta1.CosmosDBAccountLocation{
					{
						LocationName:     location,
						FailoverPriority: 2,
					},
				},
			}))
	})
	t.Run("NotEqualEnableAutomaticFailover", func(t *testing.T) {
		assert.False(t, CheckEqualDatabaseProperties(
			v1beta1.CosmosDBAccountProperties{
				EnableAutomaticFailover: azure.ToBoolPtr(true),
			},
			v1beta1.CosmosDBAccountProperties{
				EnableAutomaticFailover: azure.ToBoolPtr(false),
			}))
	})
	t.Run("EqualEnableAutomaticFailover", func(t *testing.T) {
		assert.True(t, CheckEqualDatabaseProperties(
			v1beta1.CosmosDBAccountProperties{
				EnableAutomaticFailover: azure.ToBoolPtr(true),
			},
			v1beta1.CosmosDBAccountProperties{
				EnableAutomaticFailover: azure.ToBoolPtr(true),
			}))
	})
}
