package app

import "net/url"

func (a *App) GetDisplay(location *url.URL) string {
	a.logger.With("location", location.String()).Info("get display")

	if a.user.Admin {
		return "管理员才能看到这里"
	} else {
		return "你还不是管理员"
	}
	//loc := &url.URL{
	//	Path: a.basePath,
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
