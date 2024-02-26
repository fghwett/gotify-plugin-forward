package main

import (
	"net/url"
)

func (c *MyPlugin) GetDisplay(location *url.URL) string {
	c.logger.With("location", location.String()).Info("get display")

	if c.user.Admin {
		return "You are an admin! You have super cow powers."
	} else {
		return "You are **NOT** an admin! You can do nothing:("
	}
	//loc := &url.URL{
	//	Path: c.basePath,
	//}
	//if location != nil {
	//	// If the server location can be determined, make the URL absolute
	//	loc.Scheme = location.Scheme
	//	loc.Host = location.Host
	//}
	//loc = loc.ResolveReference(&url.URL{
	//	Path: "hook",
	//})
	//return fmt.Sprintf("Set your webhook URL to %s and you are all set", loc)
}
