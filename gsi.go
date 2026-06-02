package main

import (
	"fmt"
	"strings"

	"encoding/xml"
)

type GSIScannerInfo struct {
	Mode            string              `xml:"Mode,attr"`
	VScreen         string              `xml:"V_Screen,attr"`
	MonitorList     GSIMonitorList      `xml:"MonitorList"`
	System          GSISystem           `xml:"System"`
	Department      GSIDepartment       `xml:"Department"`
	ConvFrequency   GSIConvFrequency    `xml:"ConvFrequency"`
	AGC             GSIAGC              `xml:"AGC"`
	DualWatch       GSIDualWatch        `xml:"DualWatch"`
	Property        GSIProperty         `xml:"Property"`
	ViewDescription GSIViewDescription  `xml:"ViewDescription"`
}

type GSIMonitorList struct {
	Name      string `xml:"Name,attr"`
	Index     string `xml:"Index,attr"`
	ListType  string `xml:"ListType,attr"`
	QKey      string `xml:"Q_Key,attr"`
	NTag      string `xml:"N_Tag,attr"`
	DBCounter string `xml:"DB_Counter,attr"`
}

type GSISystem struct {
	Name       string `xml:"Name,attr"`
	Index      string `xml:"Index,attr"`
	Avoid      string `xml:"Avoid,attr"`
	SystemType string `xml:"SystemType,attr"`
	QKey       string `xml:"Q_Key,attr"`
	NTag       string `xml:"N_Tag,attr"`
	Hold       string `xml:"Hold,attr"`
}

type GSIDepartment struct {
	Name  string `xml:"Name,attr"`
	Index string `xml:"Index,attr"`
	Avoid string `xml:"Avoid,attr"`
	QKey  string `xml:"Q_Key,attr"`
	Hold  string `xml:"Hold,attr"`
}

type GSIConvFrequency struct {
	Name    string `xml:"Name,attr"`
	Index   string `xml:"Index,attr"`
	Avoid   string `xml:"Avoid,attr"`
	Freq    string `xml:"Freq,attr"`
	Mod     string `xml:"Mod,attr"`
	NTag    string `xml:"N_Tag,attr"`
	Hold    string `xml:"Hold,attr"`
	SvcType string `xml:"SvcType,attr"`
	PCh     string `xml:"P_Ch,attr"`
	SAS     string `xml:"SAS,attr"`
	SAD     string `xml:"SAD,attr"`
	RecSlot string `xml:"RecSlot,attr"`
	LVL     string `xml:"LVL,attr"`
	IFX     string `xml:"IFX,attr"`
	TGID    string `xml:"TGID,attr"`
	UId     string `xml:"U_Id,attr"`
}

type GSIAGC struct {
	AAGC string `xml:"A_AGC,attr"`
	DAGC string `xml:"D_AGC,attr"`
}

type GSIDualWatch struct {
	PRI string `xml:"PRI,attr"`
	CC  string `xml:"CC,attr"`
	WX  string `xml:"WX,attr"`
}

type GSIProperty struct {
	F         string `xml:"F,attr"`
	VOL       string `xml:"VOL,attr"`
	SQL       string `xml:"SQL,attr"`
	Sig       string `xml:"Sig,attr"`
	Att       string `xml:"Att,attr"`
	Rec       string `xml:"Rec,attr"`
	KeyLock   string `xml:"KeyLock,attr"`
	P25Status string `xml:"P25Status,attr"`
	Mute      string `xml:"Mute,attr"`
	Backlight string `xml:"Backlight,attr"`
	ALed      string `xml:"A_Led,attr"`
	Dir       string `xml:"Dir,attr"`
	Rssi      string `xml:"Rssi,attr"`
}

type GSIViewDescription struct {
	InfoArea1 GSITextItem `xml:"InfoAreal"`
	InfoArea2 GSITextItem `xml:"InfoArea2"`
	OverWrite GSITextItem `xml:"OverWrite"`
}

type GSITextItem struct {
	Text string `xml:"Text,attr"`
}

type GSICommand struct{}

func (c GSICommand) Send() string { return "GSI" }

func (c GSICommand) Parse(response string) (GSIScannerInfo, error) {
	start := strings.Index(response, "<ScannerInfo")
	if start == -1 {
		start = strings.Index(response, "<?xml")
		if start == -1 {
			start = 0
		}
	}
	xmlData := response[start:]
	var info GSIScannerInfo
	if err := xml.Unmarshal([]byte(xmlData), &info); err != nil {
		preview := response
		if len(preview) > 120 {
			preview = preview[:120]
		}
		return GSIScannerInfo{}, fmt.Errorf("GSI parse: %w\nraw: %q", err, preview)
	}
	return info, nil
}

func renderGSIPanel(info GSIScannerInfo) string {
	var b strings.Builder

	field := func(label, val string) {
		fmt.Fprintf(&b, " [yellow]%-12s[-] %s\n", label+":", strings.TrimSpace(val))
	}
	section := func(name string) {
		fmt.Fprintf(&b, "\n[::b]── %s[-]\n", name)
	}

	section("Scanner")
	field("Mode", info.Mode)
	field("Screen", info.VScreen)

	section("Monitor List")
	field("Name", info.MonitorList.Name)
	field("Type", info.MonitorList.ListType)
	field("DB Count", info.MonitorList.DBCounter)
	field("Q Key", info.MonitorList.QKey)
	field("N Tag", info.MonitorList.NTag)

	section("System")
	field("Name", info.System.Name)
	field("Type", info.System.SystemType)
	field("Avoid", info.System.Avoid)
	field("Hold", info.System.Hold)
	field("Q Key", info.System.QKey)
	field("N Tag", info.System.NTag)

	section("Department")
	field("Name", info.Department.Name)
	field("Avoid", info.Department.Avoid)
	field("Hold", info.Department.Hold)
	field("Q Key", info.Department.QKey)

	section("Frequency")
	field("Name", info.ConvFrequency.Name)
	field("Freq", info.ConvFrequency.Freq)
	field("Mod", info.ConvFrequency.Mod)
	field("Svc Type", info.ConvFrequency.SvcType)
	field("Avoid", info.ConvFrequency.Avoid)
	field("Hold", info.ConvFrequency.Hold)
	field("SAS", info.ConvFrequency.SAS)
	field("SAD", info.ConvFrequency.SAD)
	field("Rec Slot", info.ConvFrequency.RecSlot)
	field("LVL", info.ConvFrequency.LVL)
	field("IFX", info.ConvFrequency.IFX)
	field("TGID", info.ConvFrequency.TGID)
	field("U_Id", info.ConvFrequency.UId)
	field("P_Ch", info.ConvFrequency.PCh)
	field("N Tag", info.ConvFrequency.NTag)

	section("AGC")
	field("Analog", info.AGC.AAGC)
	field("Digital", info.AGC.DAGC)

	section("Dual Watch")
	field("PRI", info.DualWatch.PRI)
	field("CC", info.DualWatch.CC)
	field("WX", info.DualWatch.WX)

	section("Property")
	field("VOL", info.Property.VOL)
	field("SQL", info.Property.SQL)
	field("Sig", info.Property.Sig)
	field("Att", info.Property.Att)
	field("Rec", info.Property.Rec)
	field("KeyLock", info.Property.KeyLock)
	field("P25", info.Property.P25Status)
	field("Mute", info.Property.Mute)
	field("Backlight", info.Property.Backlight)
	field("A_Led", info.Property.ALed)
	field("Dir", info.Property.Dir)
	field("RSSI", info.Property.Rssi)
	field("F", info.Property.F)

	section("View")
	field("Info 1", info.ViewDescription.InfoArea1.Text)
	field("Info 2", info.ViewDescription.InfoArea2.Text)
	field("Status", info.ViewDescription.OverWrite.Text)

	return b.String()
}
