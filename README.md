# Limoncello

A simple self-hosted golang webapp to track UK alcohol units, optimised for mobile browsers.

## Running
```
cd limoncello
go run . -f units.json -p 8080
```

Then access the servers IP address via a web browser on port `8080`.
`units.json` and `8080` are the default values for file and port arguments.

## Features
* Arbitrary volume/ABV calculation, check `var volumes`
* Weekly and monthly views
* Coloured tiles
* No external dependencies