package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/noshto/dsig"
	"github.com/noshto/dsig/pkg/safenet"
	"github.com/noshto/gen"
	"github.com/noshto/iic"
	"github.com/noshto/pdf"
	"github.com/noshto/reg"
	"github.com/noshto/sep"
)

// Configs
var (
	SepConfig     *sep.Config
	SafenetConfig *safenet.Config
)

func main() {

	loadConfig()
	loadSafenetConfig()

	PrintUsage()

	stringValue := gen.Scan("Izaberite općiju: ")
	uint64Value, err := strconv.ParseUint(stringValue, 10, 64)
	if err != nil {
		log.Fatalln(err)
	}
	switch uint64Value {
	case 1:
		registerInvoice()
	case 2:
		generateIIC()
	case 3:
		registerTCR()
	case 4:
		registerClient()
	}
}

// PrintUsage prints welcome message
func PrintUsage() {
	fmt.Println("---------------------------------------------------------------")
	fmt.Println("Welcome to FISC - simple util that saves you from frustrating process of invoices fiscalization!")
	fmt.Println("---------------------------------------------------------------")
	fmt.Println("This app is intented to help you with generation of an invoice request that meets efi.tax.gov.me fiscalization service requirements.")
	fmt.Println("You will be asked to answer the minimal list of questions sufficient for invoice fiscalization.")
	fmt.Println()
	fmt.Println("Izaberite općiju:")
	fmt.Println("[1] REGISTRACIJA I FISKALIZACIJA RAČUNA")
	fmt.Println("[2] VERIFIKACIJA IKOF")
	fmt.Println("[3] REGISTRACIJA ENU")
	fmt.Println("[4] REGISTRACIJA KLIJENATA")
}

func showErrorAndExit(err error) {
	fmt.Println(err)
	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli: ")
	os.Exit(0)
}

func registerInvoice() {
	loadConfig()
	loadSafenetConfig()

	InternalOrdNum, err := gen.GenerateRegisterInvoiceRequest(&gen.Params{
		SepConfig: SepConfig,
		OutFile:   "./gen.xml",
	})
	if err != nil {
		showErrorAndExit(err)
	}

	fmt.Println()
	fmt.Println("Molim provjerite svi podatke prije slanja u poresku!")
	fmt.Println()
	gen.PrintInvoiceDetails("./gen.xml", SepConfig, InternalOrdNum)

	fmt.Println("Nastavite sa slanjem")
	fmt.Println("[1] Da")
	fmt.Println("[2] Ne")
	stringValue := gen.Scan("Nastavite sa slanjem: ")
	uintValue, err := strconv.ParseUint(stringValue, 10, 64)
	if err != nil {
		showErrorAndExit(err)
	}
	if uintValue != 1 {
		showErrorAndExit(fmt.Errorf("slanje otkazano"))
	}

	fmt.Println("Nastavi sa slanjem")
	fmt.Print("Generisanje JIKR: ")
	if err := iic.WriteIIC(&iic.Params{
		SafenetConfig: SafenetConfig,
		InFile:        "./gen.xml",
		OutFile:       "./iic.xml",
	}); err != nil {
		showErrorAndExit(err)
	}
	fmt.Println("OK")

	fmt.Print("Generisanje DSIG: ")
	if err := dsig.Sign(&dsig.Params{
		SepConfig:     SepConfig,
		SafenetConfig: SafenetConfig,
		InFile:        "./iic.xml",
		OutFile:       "./dsig.xml",
	}); err != nil {
		showErrorAndExit(err)
	}
	fmt.Println("OK")

	fmt.Print("Registrovanje: ")
	if err := reg.Register(&reg.Params{
		SafenetConfig: SafenetConfig,
		SepConfig:     SepConfig,
		InFile:        "./dsig.xml",
		OutFile:       "./reg.xml",
	}); err != nil {
		showErrorAndExit(err)
	}
	fmt.Println("OK")

	fmt.Print("Generisanje PDF: ")
	if err := pdf.GeneratePDF(&pdf.Params{
		SepConfig:      SepConfig,
		InternalInvNum: InternalOrdNum,
		ReqFile:        "./dsig.xml",
		RespFile:       "./reg.xml",
		OutFile:        "./2021-01.pdf",
	}); err != nil {
		showErrorAndExit(err)
	}
	fmt.Println("OK")

	// TODO: store
	fmt.Print("Čuvanje rezultata: ")
	fmt.Println("NOT IMPLEMENTED")
}

func generateIIC() {
	loadSafenetConfig()
	iic, sig, err := iic.GenerateIIC(SafenetConfig, gen.GeneratePlainIIC())
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println()
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("IKOF: %s\n", iic)
	fmt.Printf("IKOF Podpis: %s\n", sig)
}

func registerTCR() {
	loadConfig()
	loadSafenetConfig()

	if err := gen.GenerateRegisterTCRRequest(&gen.Params{
		SepConfig: SepConfig,
		OutFile:   "./tcr.xml",
	}); err != nil {
		log.Fatalln(err)
	}

	if err := dsig.Sign(&dsig.Params{
		SepConfig:     SepConfig,
		SafenetConfig: SafenetConfig,
		InFile:        "./tcr.xml",
		OutFile:       "./tcr.dsig.xml",
	}); err != nil {
		log.Fatalln(err)
	}

	if err := reg.Register(&reg.Params{
		SafenetConfig: SafenetConfig,
		SepConfig:     SepConfig,
		InFile:        "./tcr.dsig.xml",
		OutFile:       "./tcr.reg.xml",
	}); err != nil {
		log.Fatalln(err)
	}
}

func registerClient() {

}

func loadConfig() {
	buf, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Fatalln(err)
	}
	SepConfig = &sep.Config{}
	err = json.Unmarshal(buf, &SepConfig)
	if err != nil {
		log.Fatalln(err)
	}
}

func loadSafenetConfig() {
	buf, err := ioutil.ReadFile("./safenet.json")
	if err != nil {
		log.Fatalln(err)
	}
	SafenetConfig = &safenet.Config{}
	err = json.Unmarshal(buf, &SafenetConfig)
	if err != nil {
		log.Fatalln(err)
	}
}
