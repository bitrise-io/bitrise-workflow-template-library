package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	stepmanModels "github.com/bitrise-io/stepman/models"
	"gopkg.in/yaml.v2"
)

const outputDir = "./_tmp"
const collectionDir = "./templates"
const templateSpecFileName = "template-spec.json"
const templateFileName = "template.yml"
const steplibSpecJSONURI = "https://bitrise-steplib-collection.s3.amazonaws.com/spec.json"

type step struct {
	Description string                  `json:"description"`
	Info        stepmanModels.StepModel `json:"step_info"`
}

type template struct {
	Title       string           `json:"title"`
	Lead        string           `json:"lead"`
	Summary     string           `json:"summary"`
	Description string           `json:"description"`
	Image       string           `json:"image"`
	Config      string           `json:"config"`
	Steps       map[string]*step `json:"steps"`
}

func stepParams(step string) (stepID string, stepVersion string) {
	stepID = step
	if s := strings.Split(step, "@"); len(s) > 1 {
		stepID = s[0]
		stepVersion = s[1]
	}
	return
}

func parseTemplate(templateSpec map[string]*template, templateID string) error {
	ymlFile, err := os.Open(getYMLPath(templateID))
	if err != nil {
		return err
	}
	templateSpec[templateID] = &template{}
	return yaml.NewDecoder(ymlFile).Decode(templateSpec[templateID])
}

func getYMLPath(templateID string) string {
	return filepath.Join(collectionDir, templateID, templateFileName)
}

func getSpecJSON() (steplibSpec stepmanModels.StepCollectionModel, err error) {
	resp, err := http.Get(steplibSpecJSONURI)
	if err != nil {
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return stepmanModels.StepCollectionModel{}, fmt.Errorf("nonsuccessful response statusCode: %s", steplibSpecJSONURI)
	}
	err = json.NewDecoder(resp.Body).Decode(&steplibSpec)
	return
}

func main() {
	//get steplib spec json
	steplibSpec, err := getSpecJSON()
	if err != nil {
		log.Fatal(err)
	}

	// read templates dirs(non-recursive)
	files, err := ioutil.ReadDir(collectionDir)
	if err != nil {
		log.Fatal(err)
	}

	// populate templateSpec | templateID: templateData
	templateSpec := map[string]*template{}
	for _, file := range files {
		// parse the template
		err := parseTemplate(templateSpec, file.Name())
		if err != nil {
			log.Fatal(err)
		}
		// filling step infos from spec json
		for s := range templateSpec[file.Name()].Steps {
			stepID, stepVersion := stepParams(s)
			i, idExists, versionExists := steplibSpec.GetStepVersion(stepID, stepVersion)
			if err != nil {
				log.Fatal(err)
			}
			if !idExists {
				log.Fatalf("Step doesn't exists with id: %s", stepID)
			}
			if !versionExists {
				log.Fatalf("Step doesn't exists with version: %s", stepVersion)
			}
			if templateSpec[file.Name()].Steps[s] == nil {
				templateSpec[file.Name()].Steps[s] = &step{}
			}
			templateSpec[file.Name()].Steps[s].Info = i.Step
		}
		fmt.Println("<-", file.Name())
	}

	out, err := os.Create(filepath.Join(outputDir, templateSpecFileName))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	if err := json.NewEncoder(out).Encode(templateSpec); err != nil {
		log.Fatal(err)
	}
}
