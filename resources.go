package main

// Regenerate the Windows icon, GUI manifest and version resource from the checked-in logo.
//go:generate go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64 --out rsrc --manifest gui --product-version 1.0.1.0 --file-version 1.0.1.0 --file-description "Android firmware partition extractor" --product-name LitePayloadDumper --original-filename LitePayloadDumper.exe --icon assets/logo-mark.png
