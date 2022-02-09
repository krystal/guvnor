# guvnor

* `guvnor deploy` - runs a deployment with the ONLY service
* `guvnor deploy [service-name]` - runs a deployment with the given service name
* `guvnor deploy [service-name] --tag 2.3.1-rc.2` - updates the tag to given tag and runs a deployment

* `guvnor scale [process-name] --quantity 4`
* `guvnor scale [service-name] [process-name] --quantity 4`

* `guvnor status` - show status for all services
* `guvnor status [service-name]` - shows status for a given service

* `guvnor run console` - runs a console in the only service
* `guvnor run [service-name] console` - runs a console in the named service

Things to come later...

* `guvnor history`
* `guvnor rollback`
