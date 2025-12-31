from PIL import Image
import os

source = r"d:\BuddyMate\assets\logo.png"
target = r"d:\BuddyMate\assets\logo_transparent.png"

try:
    img = Image.open(source)
    img = img.convert("RGBA")
    datas = img.getdata()

    newData = []
    for item in datas:
        # Check if pixel is near white (tolerance > 240)
        if item[0] > 240 and item[1] > 240 and item[2] > 240:
            newData.append((255, 255, 255, 0)) # Transparent
        else:
            newData.append(item)

    img.putdata(newData)
    img.save(target, "PNG")
    print(f"Success: Created {target}")
except Exception as e:
    print(f"Error: {e}")
