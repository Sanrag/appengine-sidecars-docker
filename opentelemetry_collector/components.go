// Copyright 2019 OpenTelemetry Authors
// Modifications Copyright 2020 Google Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/component/componenterror"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/processor/resourceprocessor"

	"github.com/googlecloudplatform/appengine-sidecars-docker/opentelemetry_collector/receiver/dockerstats"
	"github.com/googlecloudplatform/appengine-sidecars-docker/opentelemetry_collector/receiver/vmimageagereceiver"

	"github.com/open-telemetry/opentelemetry-collector-contrib/exporter/stackdriverexporter"
)

func components() (config.Factories, error) {
	errs := []error{}

	receivers, err := component.MakeReceiverFactoryMap(
		&dockerstats.Factory{},
		&vmimageagereceiver.Factory{},
	)
	if err != nil {
		errs = append(errs, err)
	}

	exporters, err := component.MakeExporterFactoryMap(
		&stackdriverexporter.Factory{},
	)
	if err != nil {
		errs = append(errs, err)
	}

	processors, err := component.MakeProcessorFactoryMap(
		resourceprocessor.NewFactory(),
	)
	if err != nil {
		errs = append(errs, err)
	}

	factories := config.Factories{
		Receivers:  receivers,
		Processors: processors,
		Exporters:  exporters,
	}

	return factories, componenterror.CombineErrors(errs)
}
