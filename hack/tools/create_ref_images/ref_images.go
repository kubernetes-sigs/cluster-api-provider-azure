//go:build tools
// +build tools

/*
Copyright 2023 The Kubernetes Authors.

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

// main is the main package for the open issues utility.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	azuredevops "github.com/microsoft/azure-devops-go-api/azuredevops/v7"
	"github.com/microsoft/azure-devops-go-api/azuredevops/v7/pipelines"
	"gopkg.in/yaml.v2"
	"k8s.io/utils/ptr"
)

type CommonSettings struct {
	AZCAPIExtensionURL string `yaml:"AZ_CAPI_EXTENSION_URL"`
	Cleanup            string `yaml:"CLEANUP"`
	ContainerImage     string `yaml:"CONTAINER_IMAGE"`
	Debug              string `yaml:"DEBUG"`
	PackerFlags        string `yaml:"PACKER_FLAGS"`
	PreflightChecks    string `yaml:"PREFLIGHT_CHECKS"`
	Branch             string `yaml:"BRANCH"`
}
type ImageConfigurations struct {
	OS                     string `yaml:"OS"`
	OSVersion              string `yaml:"OS_VERSION"`
	Offer                  string `yaml:"OFFER"`
	KubernetesBootstrapVer string `yaml:"KUBERNETES_BOOTSTRAP_VERSION"`
	KubernetesVersion      string `yaml:"KUBERNETES_VERSION"`
}

type Config struct {
	OrganizationURL     string `yaml:"organization_url"`
	ProjectName         string `yaml:"project_name"`
	PipelineID          int    `yaml:"pipeline_id"`
	PipelineName        string `yaml:"pipeline_name"`
	CommonSettings      `yaml:"common_settings"`
	ImageConfigurations []ImageConfigurations `yaml:"configurations"`
}

const (
	maxKeyLength int = 30
)

var (
	kvFormat = fmt.Sprintf("%%-%ds: %%s\n", maxKeyLength)
)

func main() {
	// check for Personal Access Token for Azure DevOps
	personalToken, tokenSet := os.LookupEnv("AZURE_DEVOPS_TOKEN")
	if !tokenSet {
		fmt.Println("AZURE_DEVOPS_TOKEN not set")
		os.Exit(1)
	}

	// check if the utility should be run in preview mode
	isPreviewRun := true
	isPreview, previewRunSet := os.LookupEnv("AZURE_DEVOPS_PREVIEW_RUN")
	if previewRunSet && isPreview == "false" {
		isPreviewRun = false
	}

	// Read the YAML file content
	yamlContent, err := os.ReadFile("config.yaml")
	if err != nil {
		log.Fatal("Error reading YAML file:", err)
	}

	// Unmarshal the YAML data into the pipelineConfig
	var config Config
	err = yaml.Unmarshal(yamlContent, &config)
	if err != nil {
		log.Fatal("Error unmarshaling YAML:", err)
	}

	fmt.Println()
	fmt.Println("Running the utility with the following configuration:")
	fmt.Println("------------------------------------")
	displayPipelineConfig(config)
	fmt.Println()
	displayCommonConfig(config.CommonSettings)
	fmt.Println()
	fmt.Println("------------------------------------")

	// create a map of config of each image
	// with key = OS + ":" + OSVersion + ":" + KubernetesVersion
	var imageConfigsPerFlavor = make(map[string]ImageConfigurations)
	for _, imageConfig := range config.ImageConfigurations {
		// add the config to the configMap
		// with key = OS + ":" + OSVersion + ":" + KubernetesVersion
		flavor := imageConfig.OS + ":" + imageConfig.OSVersion + ":" + imageConfig.KubernetesVersion
		imageConfigsPerFlavor[flavor] = imageConfig
		displayImageConfig(imageConfig)
		fmt.Println("-----")
		fmt.Println()
	}
	fmt.Println("------------------------------------")

	if isPreviewRun {
		fmt.Println("Running in preview mode")
		continueOrAbort()
		runPreview(config, imageConfigsPerFlavor, personalToken)
	} else {
		fmt.Println("Running in create mode")
		continueOrAbort()
		createImageJobsAndMonitor(config, imageConfigsPerFlavor, personalToken)
	}
}

func runPreview(config Config, imageConfigsPerFlavor map[string]ImageConfigurations, personalToken string) {
	originalCtx := context.Background()
	connection := azuredevops.NewPatConnection(config.OrganizationURL, personalToken)

	for flavor, imageConfig := range imageConfigsPerFlavor {
		ctx := context.WithValue(originalCtx, "Flavor", flavor)
		fmt.Println("------------------------------------")
		fmt.Println("Previewing image rub job for config: ", flavor)
		previewImage(ctx, connection, config, imageConfig)
		fmt.Println("------------------------------------")
	}
}

func previewImage(ctx context.Context, connection *azuredevops.Connection, config Config, imageConfigsPerFlavor ImageConfigurations) {
	pipelinesClient := pipelines.NewClient(ctx, connection)
	runPipelinesVars := getPipelinesRunVars(config, imageConfigsPerFlavor)

	previewArgs := pipelines.PreviewArgs{
		Project:    &config.ProjectName,
		PipelineId: &config.PipelineID,
		RunParameters: &pipelines.RunPipelineParameters{
			Variables: &runPipelinesVars,
			Resources: &pipelines.RunResourcesParameters{
				Repositories: &map[string]pipelines.RepositoryResourceParameters{
					"self": {
						RefName: ptr.To(config.Branch),
					},
				},
			},
		},
	}

	fmt.Println("Creating a new run for pipeline::", config.PipelineName, " pipelineID::", config.PipelineID)
	previewImageJob, err := pipelinesClient.Preview(ctx, previewArgs)
	if err != nil {
		log.Fatal(err)
	}

	// print the preview Yaml details
	fmt.Println("Details of the Preview Job")
	if previewImageJob != nil && previewImageJob.FinalYaml != nil {
		decoder := yaml.NewDecoder(strings.NewReader(*previewImageJob.FinalYaml))
		var target interface{}
		if err := decoder.Decode(&target); err != nil {
			log.Fatal("Error decoding YAML:", err)
		}

		yamlBytes, err := yaml.Marshal(target)
		if err != nil {
			log.Fatal("Error marshaling YAML:", err)
		}
		fmt.Println(string(yamlBytes))
	}
}

func createImageJobsAndMonitor(config Config, imageConfigsPerFlavor map[string]ImageConfigurations, personalToken string) {
	originalCtx := context.Background()
	connection := azuredevops.NewPatConnection(config.OrganizationURL, personalToken)

	// Create image jobs and get the run IDs
	imageRunIDs := make(map[string]int)
	for flavor, imageConfig := range imageConfigsPerFlavor {
		fmt.Println("------------------------------------")
		fmt.Println("Creating image for config: ", flavor)
		ctx := context.WithValue(originalCtx, "Flavor", flavor)
		runID := createImage(ctx, connection, config, imageConfig)
		imageRunIDs[flavor] = runID
		fmt.Println()
		fmt.Println("------------------------------------")
	}

	var wg sync.WaitGroup

	// Check the status of the image jobs
	for flavor, runID := range imageRunIDs {
		fmt.Println("------------------------------------")
		fmt.Println("Checking status of image job: ", flavor)
		ctx := context.WithValue(originalCtx, "Flavor", flavor)
		wg.Add(1)
		go func(ctx context.Context, connection *azuredevops.Connection, config Config, runID int, flavor string) {
			defer wg.Done()
			checkJobStatus(ctx, connection, config, runID, flavor)
		}(ctx, connection, config, runID, flavor)
		// go checkJobStatus(job.ctx, connection, config, job.runID, flavor)
		fmt.Println("------------------------------------")
	}
	wg.Wait()
}

func createImage(ctx context.Context, connection *azuredevops.Connection, config Config, imageConfig ImageConfigurations) int {
	pipelinesClient := pipelines.NewClient(ctx, connection)
	runPipelinesVars := getPipelinesRunVars(config, imageConfig)

	createNewImageRunArgs := pipelines.RunPipelineArgs{
		Project:    &config.ProjectName,
		PipelineId: &config.PipelineID,
		RunParameters: &pipelines.RunPipelineParameters{
			Variables: &runPipelinesVars,
			Resources: &pipelines.RunResourcesParameters{
				Repositories: &map[string]pipelines.RepositoryResourceParameters{
					"self": {
						RefName: ptr.To(config.Branch),
					},
				},
			},
		},
	}

	fmt.Println("Creating a new run for pipeline::", config.PipelineName, " pipelineID::", config.PipelineID)
	createNewImageRun, err := pipelinesClient.RunPipeline(ctx, createNewImageRunArgs)
	if err != nil {
		if createNewImageRun != nil {
			return *createNewImageRun.Id
		}
		log.Fatal(err)
	}

	// print the run details
	fmt.Println("Details of the created run")
	displayRunDetails(createNewImageRun)

	return *createNewImageRun.Id
}

func checkJobStatus(ctx context.Context, connection *azuredevops.Connection, pipelineConfig Config, runID int, flavor string) {
	pipelinesClient := pipelines.NewClient(ctx, connection)
	// fmt.Println("Getting run details")
	runningImageJob, err := pipelinesClient.GetRun(ctx, pipelines.GetRunArgs{
		Project:    &pipelineConfig.ProjectName,
		PipelineId: &pipelineConfig.PipelineID,
		RunId:      &runID,
	})
	if err != nil {
		log.Fatal(err)
	}

	// run until the job is complete
	for *runningImageJob.State == pipelines.RunStateValues.InProgress {
		fmt.Println("Run is still in progress for pipeline:", pipelineConfig.ProjectName, "pipelineID:", pipelineConfig.PipelineID, "flavor:", flavor, "runID:", runID, "Job Name:")
		time.Sleep(60 * time.Second)
		runningImageJob, err = pipelinesClient.GetRun(ctx, pipelines.GetRunArgs{
			Project:    &pipelineConfig.ProjectName,
			PipelineId: &pipelineConfig.PipelineID,
			RunId:      &runID,
		})
		if err != nil {
			log.Fatal(err)
		}
	}

	if *runningImageJob.State == pipelines.RunStateValues.Completed {
		fmt.Println("Run completed for pipeline:", pipelineConfig.ProjectName, "pipelineID:", pipelineConfig.PipelineID, "flavor:", flavor, "runID:", runID, "Job Name:", *runningImageJob.Name, "Job State:", *runningImageJob.State, "Job Result:", *runningImageJob.Result)
	} else {
		fmt.Println("Run failed/cancelled/unknown for pipeline:", pipelineConfig.ProjectName, "pipelineID:", pipelineConfig.PipelineID, "flavor:", flavor, "runID:", runID, "Job Name:", *runningImageJob.Name, "Job State:", *runningImageJob.State, "Job Result:", *runningImageJob.Result)
	}

	// displayRunDetails(runningImageJob)
}

func getPipelinesRunVars(config Config, imageConfig ImageConfigurations) map[string]pipelines.Variable {
	return map[string]pipelines.Variable{
		"AZ_CAPI_EXTENSION_URL":        {Value: ptr.To(config.AZCAPIExtensionURL)},
		"CLEANUP":                      {Value: ptr.To(config.Cleanup)},
		"CONTAINER_IMAGE":              {Value: ptr.To(config.ContainerImage)},
		"DEBUG":                        {Value: ptr.To(config.Debug)},
		"PACKER_FLAGS":                 {Value: ptr.To(config.PackerFlags)},
		"PREFLIGHT_CHECKS":             {Value: ptr.To(config.PreflightChecks)},
		"KUBERNETES_BOOTSTRAP_VERSION": {Value: ptr.To(imageConfig.KubernetesBootstrapVer)},
		"KUBERNETES_VERSION":           {Value: ptr.To(imageConfig.KubernetesVersion)},
		"OFFER":                        {Value: ptr.To(imageConfig.Offer)},
		"OS":                           {Value: ptr.To(imageConfig.OS)},
		"OS_VERSION":                   {Value: ptr.To(imageConfig.OSVersion)},
	}
}

func continueOrAbort() {
	var response string
	fmt.Println()
	fmt.Println("Continue? (y/n)")
	fmt.Scanln(&response)
	if response != "y" {
		os.Exit(0)
	}
	fmt.Println()
}

func displayRunDetails(run *pipelines.Run) {
	if run != nil {
		if run.Id != nil {
			fmt.Printf(kvFormat, "ID", strconv.Itoa(*run.Id))
		}
		if run.Name != nil {
			fmt.Printf(kvFormat, "Name", *run.Name)
		}
		if run.CreatedDate != nil {
			fmt.Printf(kvFormat, "CreatedDate", *run.CreatedDate)
		}
		if run.State != nil {
			fmt.Printf(kvFormat, "State", *run.State)
		}
		if run.Result != nil {
			fmt.Printf(kvFormat, "Result", *run.Result)
		}
		if run.Pipeline != nil {
			if run.Pipeline.Id != nil {
				fmt.Printf(kvFormat, "Pipeline", strconv.Itoa(*run.Pipeline.Id))
			}
			if run.Pipeline.Name != nil {
				fmt.Printf(kvFormat, "Pipeline Name", *run.Pipeline.Name)
			}
			if run.Pipeline.Url != nil {
				fmt.Printf(kvFormat, "Pipeline URL", *run.Pipeline.Url)
			}
		}
		if run.FinishedDate != nil {
			fmt.Printf(kvFormat, "FinishedDate", *run.FinishedDate)
		}
		if run.Resources != nil {
			if run.Resources.Repositories != nil {
				fmt.Printf(kvFormat, "Repositories")
				for str, repo := range *run.Resources.Repositories {
					fmt.Printf(kvFormat, "  ", str)
					if repo.RefName != nil {
						fmt.Printf(kvFormat, "  repo.RefName", *repo.RefName)
					}
					if repo.Version != nil {
						fmt.Printf(kvFormat, "  repo.Version", *repo.Version)
					}
					if repo.Repository != nil {
						if repo.Repository.Type != nil {
							fmt.Printf(kvFormat, "  repo.Repository.Type", *repo.Repository.Type)
						}
					}
				}
			}
		}
	}
}

func displayPipelineConfig(config Config) {
	fmt.Printf(kvFormat, "OrganizationURL", config.OrganizationURL)
	fmt.Printf(kvFormat, "ProjectName", config.ProjectName)
	fmt.Printf(kvFormat, "PipelineName", config.PipelineName)
	fmt.Printf(kvFormat, "PipelineID", strconv.Itoa(config.PipelineID))
}

func displayCommonConfig(commonSetting CommonSettings) {
	fmt.Printf(kvFormat, "AZCAPIExtensionURL", commonSetting.AZCAPIExtensionURL)
	fmt.Printf(kvFormat, "Cleanup", commonSetting.Cleanup)
	fmt.Printf(kvFormat, "ContainerImage", commonSetting.ContainerImage)
	fmt.Printf(kvFormat, "Debug", commonSetting.Debug)
	fmt.Printf(kvFormat, "PackerFlags", commonSetting.PackerFlags)
	fmt.Printf(kvFormat, "PreflightChecks", commonSetting.PreflightChecks)
	fmt.Printf(kvFormat, "Branch", commonSetting.Branch)
}

func displayImageConfig(imageConfig ImageConfigurations) {
	fmt.Printf(kvFormat, "OS", imageConfig.OS)
	fmt.Printf(kvFormat, "OSVersion", imageConfig.OSVersion)
	fmt.Printf(kvFormat, "Offer", imageConfig.Offer)
	fmt.Printf(kvFormat, "KubernetesBootstrapVer", imageConfig.KubernetesBootstrapVer)
	fmt.Printf(kvFormat, "KubernetesVersion", imageConfig.KubernetesVersion)
}
