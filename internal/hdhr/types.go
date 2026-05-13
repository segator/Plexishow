package hdhr

// DeviceDescription matches /device.xml UPnP response.
type DeviceDescription struct {
	XMLName      string `xml:"root"`
	SpecMajor    int    `xml:"specVersion>major"`
	SpecMinor    int    `xml:"specVersion>minor"`
	URLBase      string `xml:"URLBase"`
	DeviceType   string `xml:"device>deviceType"`
	FriendlyName string `xml:"device>friendlyName"`
	Manufacturer string `xml:"device>manufacturer"`
	ModelNumber  string `xml:"device>modelNumber"`
	ModelName    string `xml:"device>modelName"`
	SerialNumber string `xml:"device>serialNumber"`
}

type Discover struct {
	FriendlyName    string `json:"FriendlyName"`
	Manufacturer    string `json:"Manufacturer"`
	ModelNumber     string `json:"ModelNumber"`
	FirmwareName    string `json:"FirmwareName"`
	TunerCount      int    `json:"TunerCount"`
	FirmwareVersion string `json:"FirmwareVersion"`
	DeviceID        string `json:"DeviceID"`
	DeviceAuth      string `json:"DeviceAuth"`
	BaseURL         string `json:"BaseURL"`
	LineupURL       string `json:"LineupURL"`
}

type LineupStatus struct {
	ScanInProgress int      `json:"ScanInProgress"`
	ScanPossible   int      `json:"ScanPossible"`
	Source         string   `json:"Source"`
	SourceList     []string `json:"SourceList"`
}

type LineupItem struct {
	GuideNumber string `json:"GuideNumber"`
	GuideName   string `json:"GuideName"`
	URL         string `json:"URL"`
	HD          int    `json:"HD,omitempty"`
	HDHRURL     string `json:"HDHomeRunURL,omitempty"`
}
