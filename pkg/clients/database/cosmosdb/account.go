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
	"encoding/json"
	"fmt"
	"sort"

	"github.com/Azure/azure-sdk-for-go/services/cosmos-db/mgmt/2015-04-08/documentdb"
	"github.com/Azure/azure-sdk-for-go/services/cosmos-db/mgmt/2015-04-08/documentdb/documentdbapi"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/pkg/errors"

	"github.com/crossplane/provider-azure/apis/database/v1beta1"
	azure "github.com/crossplane/provider-azure/pkg/clients"
)

// A AccountClient handles CRUD operations for Azure CosmosDB Accounts.
type AccountClient documentdbapi.DatabaseAccountsClientAPI

// NewDatabaseAccountClient create Azure DatabaseAccountsClient using provided credentials data
func NewDatabaseAccountClient(credentials []byte) (AccountClient, error) {
	creds := &azure.Credentials{}
	if err := json.Unmarshal(credentials, creds); err != nil {
		return nil, errors.Wrap(err, "cannot unmarshal Azure client secret data")
	}

	config := auth.NewClientCredentialsConfig(creds.ClientID, creds.ClientSecret, creds.TenantID)
	config.AADEndpoint = creds.ActiveDirectoryEndpointURL
	config.Resource = creds.ResourceManagerEndpointURL

	authorizer, err := config.Authorizer()
	if err != nil {
		return nil, fmt.Errorf("failed to get authorizer from config: %+v", err)
	}

	client := documentdb.NewDatabaseAccountsClient(creds.SubscriptionID)
	client.Authorizer = authorizer

	if err := client.AddToUserAgent(azure.UserAgent); err != nil {
		return nil, errors.Wrap(err, "cannot add to Azure client user agent")
	}

	return client, nil
}

// ToDatabaseAccountCreate from DatabaseAccountSpec
func ToDatabaseAccountCreate(s *v1beta1.CosmosDBAccountSpec) documentdb.DatabaseAccountCreateUpdateParameters {
	if s == nil {
		return documentdb.DatabaseAccountCreateUpdateParameters{}
	}

	//DatabaseAccountProperties
	return documentdb.DatabaseAccountCreateUpdateParameters{
		Kind:                                  s.ForProvider.Kind,
		Location:                              azure.ToStringPtr(s.ForProvider.Location),
		Tags:                                  azure.ToStringPtrMap(s.ForProvider.Tags),
		DatabaseAccountCreateUpdateProperties: toDatabaseProperties(&s.ForProvider.Properties),
	}
}

// ToDatabaseAccountUpdate from DatabaseAccountSpec
func ToDatabaseAccountUpdate(s *v1beta1.CosmosDBAccountSpec) documentdb.DatabaseAccountCreateUpdateParameters {
	return ToDatabaseAccountCreate(s)
}

// UpdateCosmosDBAccountObservation produces SQLServerObservation from documentdb.DatabaseAccount.
func UpdateCosmosDBAccountObservation(o *v1beta1.CosmosDBAccountStatus, in documentdb.DatabaseAccount) {
	o.AtProvider = &v1beta1.CosmosDBAccountObservation{
		ID:         azure.ToString(in.ID),
		State:      azure.ToString(in.DatabaseAccountProperties.ProvisioningState),
		Properties: fromDatabaseProperties(in.DatabaseAccountProperties),
		Location:   azure.ToString(in.Location),
		Tags:       azure.ToStringMap(in.Tags),
	}
}

func toDatabaseProperties(a *v1beta1.CosmosDBAccountProperties) *documentdb.DatabaseAccountCreateUpdateProperties {
	if a == nil {
		return nil
	}

	return &documentdb.DatabaseAccountCreateUpdateProperties{
		ConsistencyPolicy:            toDatabaseConsistencyPolicy(a.ConsistencyPolicy),
		Locations:                    toDatabaseLocations(a.Locations),
		DatabaseAccountOfferType:     azure.ToStringPtr(a.DatabaseAccountOfferType),
		EnableAutomaticFailover:      a.EnableAutomaticFailover,
		EnableCassandraConnector:     a.EnableCassandraConnector,
		EnableMultipleWriteLocations: a.EnableAutomaticFailover,
	}
}

func fromDatabaseProperties(a *documentdb.DatabaseAccountProperties) v1beta1.CosmosDBAccountProperties {
	if a == nil {
		return v1beta1.CosmosDBAccountProperties{}
	}

	// TODO(asouza): figure out how to handle WriteLocations since Create request do not have R/W Locations, only Locations.
	return v1beta1.CosmosDBAccountProperties{
		ConsistencyPolicy:            fromDatabaseConsistencyPolicy(a.ConsistencyPolicy),
		Locations:                    fromDatabaseLocations(a.ReadLocations),
		DatabaseAccountOfferType:     string(a.DatabaseAccountOfferType),
		EnableAutomaticFailover:      a.EnableAutomaticFailover,
		EnableCassandraConnector:     a.EnableCassandraConnector,
		EnableMultipleWriteLocations: a.EnableAutomaticFailover,
	}
}

// CheckEqualDatabaseProperties compares the observed state with the desired spec.
func CheckEqualDatabaseProperties(p v1beta1.CosmosDBAccountProperties, o v1beta1.CosmosDBAccountProperties) bool {
	// asouza: only keep attributes that can be modified in the comparison.
	return (equalConsistencyPolicyIfNotNull(p.ConsistencyPolicy, o.ConsistencyPolicy) &&
		checkEqualLocations(p.Locations, o.Locations) &&
		equalBoolIfNotNull(p.EnableAutomaticFailover, o.EnableAutomaticFailover) &&
		equalBoolIfNotNull(p.EnableMultipleWriteLocations, o.EnableMultipleWriteLocations))
}

func equalConsistencyPolicyIfNotNull(spec, current *v1beta1.CosmosDBAccountConsistencyPolicy) bool {
	if spec != nil {
		return (spec == current) || (*spec == *current)
	}

	return true
}

func equalBoolIfNotNull(spec, current *bool) bool {
	return azure.ToBool(spec) == azure.ToBool(current)
}

func toDatabaseConsistencyPolicy(a *v1beta1.CosmosDBAccountConsistencyPolicy) *documentdb.ConsistencyPolicy {
	if a == nil {
		return nil
	}

	return &documentdb.ConsistencyPolicy{
		DefaultConsistencyLevel: documentdb.DefaultConsistencyLevel(a.DefaultConsistencyLevel),
		MaxStalenessPrefix:      a.MaxStalenessPrefix,
		MaxIntervalInSeconds:    a.MaxIntervalInSeconds,
	}
}

func fromDatabaseConsistencyPolicy(a *documentdb.ConsistencyPolicy) *v1beta1.CosmosDBAccountConsistencyPolicy {
	if a == nil {
		return nil
	}

	return &v1beta1.CosmosDBAccountConsistencyPolicy{
		DefaultConsistencyLevel: string(a.DefaultConsistencyLevel),
		MaxStalenessPrefix:      a.MaxStalenessPrefix,
		MaxIntervalInSeconds:    a.MaxIntervalInSeconds,
	}
}

func toDatabaseLocations(a []v1beta1.CosmosDBAccountLocation) *[]documentdb.Location {
	if a == nil {
		return &[]documentdb.Location{}
	}

	s := make([]documentdb.Location, len(a))
	for i, location := range a {
		s[i] = documentdb.Location{
			LocationName:     &location.LocationName,
			FailoverPriority: &location.FailoverPriority,
			IsZoneRedundant:  &location.IsZoneRedundant,
		}
	}

	return &s
}

func fromDatabaseLocations(a *[]documentdb.Location) []v1beta1.CosmosDBAccountLocation {
	lenA := 0
	if a != nil {
		lenA = len(*a)
	}
	s := make([]v1beta1.CosmosDBAccountLocation, lenA)
	if lenA > 0 {
		for i, location := range *a {
			s[i] = v1beta1.CosmosDBAccountLocation{
				LocationName:     *location.LocationName,
				FailoverPriority: *location.FailoverPriority,
				IsZoneRedundant:  *location.IsZoneRedundant,
			}
		}
	}
	return s
}

func sortLocations(a []v1beta1.CosmosDBAccountLocation) {
	sort.SliceStable(a, func(i, j int) bool {
		return a[i].LocationName < a[j].LocationName
	})
}

func checkEqualLocations(a, b []v1beta1.CosmosDBAccountLocation) bool {
	sortLocations(a)
	sortLocations(b)

	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i].LocationName != b[i].LocationName {
			return false
		}
		if a[i].FailoverPriority != b[i].FailoverPriority {
			return false
		}
		if a[i].IsZoneRedundant != b[i].IsZoneRedundant {
			return false
		}
	}

	return true
}
