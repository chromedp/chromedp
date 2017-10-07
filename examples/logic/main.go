// examples/logic/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cdp "github.com/knq/chromedp"
	cdptypes "github.com/knq/chromedp/cdp"
)

func main() {
	var err error

	// create context
	ctxt, cancel := context.WithCancel(context.Background())
	defer cancel()

	// create chrome instance
	c, err := cdp.New(ctxt, cdp.WithLog(log.Printf))
	if err != nil {
		log.Fatal(err)
	}

	// list awesome go projects for the "Selenium and browser control tools."
	res, err := listAwesomeGoProjects(ctxt, c, "Selenium and browser control tools.")
	if err != nil {
		log.Fatalf("could not list awesome go projects: %v", err)
	}

	// shutdown chrome
	err = c.Shutdown(ctxt)
	if err != nil {
		log.Fatal(err)
	}

	// wait for chrome to finish
	err = c.Wait()
	if err != nil {
		log.Fatal(err)
	}

	// output the values
	for k, v := range res {
		log.Printf("project %s (%s): '%s'", k, v.URL, v.Description)
	}
}

// ud contains a url, description for a project.
type ud struct {
	URL, Description string
}

// listAwesomeGoProjects is the highest level logic for browsing to the
// awesome-go page, finding the specified section sect, and retrieving the
// associated projects from the page.
func listAwesomeGoProjects(ctxt context.Context, c *cdp.CDP, sect string) (map[string]ud, error) {
	// force max timeout of 15 seconds for retrieving and processing the data
	var cancel func()
	ctxt, cancel = context.WithTimeout(ctxt, 25*time.Second)
	defer cancel()

	sel := fmt.Sprintf(`//p[text()[contains(., '%s')]]`, sect)

	// navigate
	if err := c.Run(ctxt, cdp.Navigate(`https://github.com/avelino/awesome-go`)); err != nil {
		return nil, fmt.Errorf("could not navigate to github: %v", err)
	}

	// wait visible
	if err := c.Run(ctxt, cdp.WaitVisible(sel)); err != nil {
		return nil, fmt.Errorf("could not get section: %v", err)
	}

	sib := sel + `/following-sibling::ul/li`

	// get project link text
	var projects []*cdptypes.Node
	if err := c.Run(ctxt, cdp.Nodes(sib+`/child::a/text()`, &projects)); err != nil {
		return nil, fmt.Errorf("could not get projects: %v", err)
	}

	// get links and description text
	var linksAndDescriptions []*cdptypes.Node
	if err := c.Run(ctxt, cdp.Nodes(sib+`/child::node()`, &linksAndDescriptions)); err != nil {
		return nil, fmt.Errorf("could not get links and descriptions: %v", err)
	}

	// check length
	if 2*len(projects) != len(linksAndDescriptions) {
		return nil, fmt.Errorf("projects and links and descriptions lengths do not match (2*%d != %d)", len(projects), len(linksAndDescriptions))
	}

	// process data
	res := make(map[string]ud)
	for i := 0; i < len(projects); i++ {
		res[projects[i].NodeValue] = ud{
			URL:         linksAndDescriptions[2*i].AttributeValue("href"),
			Description: strings.TrimPrefix(strings.TrimSpace(linksAndDescriptions[2*i+1].NodeValue), "- "),
		}
	}

	return res, nil
}
