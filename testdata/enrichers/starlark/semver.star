#!/usr/bin/env star

def enrich(input):
    data = input.payload

    for pkg in data.Packages:
        if pkg.Name == "log4j-api" and semver(pkg.Version, "<= 2.14.1"):
            return {
                "critical": "true",
                "reason": "Log4j vulnerability",
            }

    return {
        "critical": "false",        
    }