#!/usr/bin/env star

# Return summary of findings
def enrich(input):
    data = input.payload
    waived = "false"
    namespace = data.Image.ContextRoots[0].Contexts[0].Data.namespace
    if namespace == "kube-system":
       waived = "true"
    if namespace == "cattle-system":
       waived = "true"
    if namespace == "cattle-fleet-system":
       waived = "true"
    if namespace == "cattle-monitoring-system":
       waived = "true"
    return { 
        "waived": waived,
    }

