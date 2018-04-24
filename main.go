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

	"gopkg.in/yaml.v2"
)

const outputDir = "./_tmp"
const collectionDir = "./templates"
const specFileName = "template-spec.json"
const templateFileName = "template.yml"
const steplibSpecJSONURI = "https://bitrise-steplib-collection.s3.amazonaws.com/spec.json"

type step struct {
	Description string      `json:"description"`
	Info        interface{} `json:"step_info"`
}

type template struct {
	Title       string           `json:"title"`
	Lead        string           `json:"lead"`
	Summary     string           `json:"summary"`
	Description string           `json:"description"`
	Image       string           `json:"image"`
	Yaml        string           `json:"yaml"`
	Steps       map[string]*step `json:"steps"`
}

type steplibSpec struct {
	Steps map[string]struct {
		LatestVersion string                 `json:"latest_version_number"`
		Versions      map[string]interface{} `json:"versions"`
	}
}

func (slibSpec steplibSpec) fetchVersion(stepID string) (interface{}, error) {
	s := strings.Split(stepID, "@")

	step, ok := slibSpec.Steps[s[0]]
	if !ok {
		return nil, fmt.Errorf("no step found with id: %s", s[0])
	}

	if len(s) == 2 {
		stepInfo, ok := step.Versions[s[1]]
		if !ok {
			return nil, fmt.Errorf("no version(%s) found for stepID: %s", s[1], s[0])
		}
		return stepInfo, nil
	}

	v, ok := step.Versions[step.LatestVersion]
	if !ok {
		return nil, fmt.Errorf("no version(%s) found for stepID: %s", step.LatestVersion, s[0])
	}

	return v, nil
}

func main() {
	//get steplib spec json
	slibSpec, err := getSpecJSON()
	if err != nil {
		log.Fatal(err)
	}

	// read templates dirs(non-recursive)
	files, err := ioutil.ReadDir(collectionDir)
	if err != nil {
		log.Fatal(err)
	}

	// populate tplSpec | templateID: templateData
	tplSpec := map[string]*template{}
	for _, file := range files {
		// parse the template
		err := parseTemplate(tplSpec, file.Name())
		if err != nil {
			log.Fatal(err)
		}
		// filling step infos from spec json
		for stepID := range tplSpec[file.Name()].Steps {
			i, err := slibSpec.fetchVersion(stepID)
			if err != nil {
				log.Fatal(err)
			}
			if tplSpec[file.Name()].Steps[stepID] == nil {
				tplSpec[file.Name()].Steps[stepID] = &step{}
			}
			tplSpec[file.Name()].Steps[stepID].Info = i
		}
		fmt.Println("<-", file.Name())
	}

	out, err := os.Create(filepath.Join(outputDir, specFileName))
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Fatal(err)
		}
	}()

	err = json.NewEncoder(out).Encode(tplSpec)
	if err != nil {
		log.Fatal(err)
	}
}

func parseTemplate(tplSpec map[string]*template, templateID string) error {
	ymlFile, err := os.Open(getYMLPath(templateID))
	if err != nil {
		return err
	}
	tplSpec[templateID] = &template{}
	return yaml.NewDecoder(ymlFile).Decode(tplSpec[templateID])
}

func getYMLPath(templateID string) string {
	return filepath.Join(collectionDir, templateID, templateFileName)
}

func getSpecJSON() (slibSpec steplibSpec, err error) {
	resp, err := http.Get(steplibSpecJSONURI)
	if err != nil {
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return steplibSpec{}, fmt.Errorf("non-successful response statusCode: %s", steplibSpecJSONURI)
	}
	err = json.NewDecoder(resp.Body).Decode(&slibSpec)
	if err != nil {
		return
	}
	return
}
