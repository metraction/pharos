#!/usr/bin/env star

# Print basic information
print("image:")
print("------------------------------")
print("I can reach flat Version: " + str(payload["Version"]))
# print("But here I fail : " + payload.Image.ImageSpec)

# Access Image data directly
def calculate_risk():
    # Get the image data
    image = payload["Image"]
    
    # Get the ImageSpec
    image_spec = image["ImageSpec"]
    print("ImageSpec: " + str(image_spec))
    
    # Calculate risk based on ImageSpec
    if image_spec != "":
        return 1
    return 0

# Calculate risk
risk = calculate_risk()