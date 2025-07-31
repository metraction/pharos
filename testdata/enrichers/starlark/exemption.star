#!/usr/bin/env star

# Function to check if an image is exempted
def enrich(input):
    data = input.payload
    if matches("corda.*", data.Image.ImageSpec):
        return { 
            "image": data.Image.ImageSpec,
            "exempted": True,
            "reason": "Corda image has jar files that are not used, but can't be removed"
         }
    else:
        return { 
            "image": data.Image.ImageSpec,
            "exempted": False,
            "reason": "No exemption found"
         }

