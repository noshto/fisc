package main

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/beevik/etree"
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
	Clients       = &[]sep.Client{}
	SepConfig     = &sep.Config{}
	SafenetConfig = &safenet.Config{}
)

func main() {
	if err := loadConfig(); err != nil {
		registerCompany()
	}
	if err := loadConfig(); err != nil {
		showErrorAndExit(err)
	}

	if SepConfig.TCR == nil {
		if err := registerTCR(); err != nil {
			showErrorAndExit(err)
		}
	}

	PrintUsage()

	stringValue := gen.Scan("Izaberite općiju: ")
	uint64Value, err := strconv.ParseUint(stringValue, 10, 64)
	if err != nil {
		log.Fatalln(err)
	}
	switch uint64Value {
	case 1:
		if err := registerInvoice(); err != nil {
			showErrorAndExit(err)
		}
	case 2:
		if err := generateIIC(); err != nil {
			showErrorAndExit(err)
		}
	case 3:
		if err := registerTCR(); err != nil {
			showErrorAndExit(err)
		}
	case 4:
		if err := registerClient(); err != nil {
			showErrorAndExit(err)
		}
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

func registerInvoice() error {
	loadClients()
	if err := loadSafenetConfig(); err != nil {
		if err := setSafenetConfig(); err != nil {
			return err
		}
	}

	InternalOrdNum, err := gen.GenerateRegisterInvoiceRequest(&gen.Params{
		SepConfig: SepConfig,
		Clients:   Clients,
		OutFile:   currentWorkingDirectoryFilePath("gen.xml"),
	})
	if err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("Molim provjerite svi podatke prije slanja u poresku!")
	fmt.Println()
	gen.PrintInvoiceDetails(currentWorkingDirectoryFilePath("gen.xml"), SepConfig, Clients, InternalOrdNum)

	fmt.Println("Nastavite sa slanjem")
	fmt.Println("[1] Da")
	fmt.Println("[2] Ne")
	stringValue := gen.Scan("Nastavite sa slanjem: ")
	uintValue, err := strconv.ParseUint(stringValue, 10, 64)
	if err != nil {
		return err
	}
	if uintValue != 1 {
		return fmt.Errorf("slanje otkazano")
	}

	fmt.Println("Nastavi sa slanjem")
	fmt.Print("Generisanje JIKR: ")
	if err := iic.WriteIIC(&iic.Params{
		SafenetConfig: SafenetConfig,
		InFile:        currentWorkingDirectoryFilePath("gen.xml"),
		OutFile:       currentWorkingDirectoryFilePath("iic.xml"),
	}); err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Print("Generisanje DSIG: ")
	if err := dsig.Sign(&dsig.Params{
		SepConfig:     SepConfig,
		SafenetConfig: SafenetConfig,
		InFile:        currentWorkingDirectoryFilePath("iic.xml"),
		OutFile:       currentWorkingDirectoryFilePath("dsig.xml"),
	}); err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Print("Registrovanje: ")
	if err := reg.Register(&reg.Params{
		SafenetConfig: SafenetConfig,
		SepConfig:     SepConfig,
		InFile:        currentWorkingDirectoryFilePath("dsig.xml"),
		OutFile:       currentWorkingDirectoryFilePath("reg.xml"),
	}); err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Print("Generisanje PDF: ")
	if err := pdf.GeneratePDF(&pdf.Params{
		SepConfig:      SepConfig,
		Clients:        Clients,
		InternalInvNum: InternalOrdNum,
		ReqFile:        currentWorkingDirectoryFilePath("dsig.xml"),
		RespFile:       currentWorkingDirectoryFilePath("reg.xml"),
		OutFile:        currentWorkingDirectoryFilePath("2021-01.pdf"),
	}); err != nil {
		return err
	}
	fmt.Println("OK")

	// TODO: store
	fmt.Print("Čuvanje rezultata: ")
	fmt.Println("NOT IMPLEMENTED")
	return nil
}

func generateIIC() error {
	if err := loadSafenetConfig(); err != nil {
		if err := setSafenetConfig(); err != nil {
			return err
		}
	}
	iic, sig, err := iic.GenerateIIC(SafenetConfig, gen.GeneratePlainIIC())
	if err != nil {
		return err
	}
	fmt.Println()
	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("IKOF: %s\n", iic)
	fmt.Printf("IKOF Podpis: %s\n", sig)

	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli: ")
	return nil
}

func currentWorkingDirectoryFilePath(fileName string) string {
	return filepath.Join(".", fileName)
}

func registerTCR() error {
	if err := loadSafenetConfig(); err != nil {
		if err := setSafenetConfig(); err != nil {
			return err
		}
	}

	if err := gen.GenerateRegisterTCRRequest(&gen.Params{
		SepConfig: SepConfig,
		OutFile:   currentWorkingDirectoryFilePath("tcr.xml"),
	}); err != nil {
		return err
	}

	if err := dsig.Sign(&dsig.Params{
		SepConfig:     SepConfig,
		SafenetConfig: SafenetConfig,
		InFile:        currentWorkingDirectoryFilePath("tcr.xml"),
		OutFile:       currentWorkingDirectoryFilePath("tcr.dsig.xml"),
	}); err != nil {
		return err
	}

	if err := reg.Register(&reg.Params{
		SafenetConfig: SafenetConfig,
		SepConfig:     SepConfig,
		InFile:        currentWorkingDirectoryFilePath("tcr.dsig.xml"),
		OutFile:       currentWorkingDirectoryFilePath("tcr.reg.xml"),
	}); err != nil {
		return err
	}

	buf, err := ioutil.ReadFile(currentWorkingDirectoryFilePath("tcr.reg.xml"))
	if err != nil {
		return err
	}
	RegisterTCRResponse := sep.RegisterTCRResponse{}
	if err := xml.Unmarshal(buf, &RegisterTCRResponse); err != nil {
		return err
	}
	if RegisterTCRResponse.Body.RegisterTCRResponse.TCRCode == "" {
		return fmt.Errorf("%v", RegisterTCRResponse.Body.Fault)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromFile(currentWorkingDirectoryFilePath("tcr.dsig.xml")); err != nil {
		return err
	}
	elem := doc.FindElement("//TCR")
	if elem == nil {
		return fmt.Errorf("invalid xml, no RegisterTCRRequest")
	}
	elemDoc := etree.NewDocument()
	elemDoc.SetRoot(elem.Copy())
	buf, err = elemDoc.WriteToBytes()
	if err != nil {
		return err
	}

	TCR := sep.TCR{}
	if err := xml.Unmarshal(buf, &TCR); err != nil {
		return err
	}
	TCR.TCRCode = string(RegisterTCRResponse.Body.RegisterTCRResponse.TCRCode)
	SepConfig.TCR = &TCR
	if err := saveSepConfig(); err != nil {
		return err
	}

	fmt.Println("Detalji ENU su uspešno registrovani i sačuvani")
	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli: ")
	return nil
}

// generateClient asks user to fill in new client details
func generateClient() *sep.Client {
	fmt.Println()
	fmt.Println("---------------------------------------------------------------")
	fmt.Println("Molim unesite podatke za novog klijenata:")
	return &sep.Client{
		Name:    gen.Scan("Ime: "),
		TIN:     gen.Scan("Identifikacioni broj (PIB): "),
		VAT:     gen.Scan("PDV broj (PDV): "),
		Address: gen.Scan("Adresa: "),
		Town:    gen.Scan("Grad: "),
		Country: gen.Scan("Država (MNE, USA, itd.): "),
	}
}

func registerClient() error {
	client := generateClient()
	if Clients == nil {
		Clients = &[]sep.Client{*client}
	} else {
		*Clients = append(*Clients, *client)
	}
	saveClients()

	fmt.Println("Detalji klijenta su uspešno sačuvani")
	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli: ")
	return nil
}

func registerCompany() {
	fmt.Println("Molim unesite detalji firme")
	cfg := sep.Config{
		Name:         gen.Scan("Naziv: "),
		TIN:          gen.Scan("Identifikacioni broj (PIB): "),
		VAT:          gen.Scan("PDV broj (PDV): "),
		Address:      gen.Scan("Adresa: "),
		Town:         gen.Scan("Grad: "),
		Country:      "MNE",
		Phone:        gen.Scan("Tel: "),
		Fax:          gen.Scan("Fax: "),
		BankAccount:  gen.Scan("Z.R.: "),
		Environment:  sep.TEST,
		OperatorCode: gen.Scan("Kod operatera: "),
	}

	buf, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		showErrorAndExit(err)
	}
	err = ioutil.WriteFile(currentWorkingDirectoryFilePath("config.json"), buf, 0644)
	if err != nil {
		showErrorAndExit(err)
	}
	fmt.Println("Detalji su uspešno sačuvani")
	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli: ")
	os.Exit(0)
}

func loadConfig() error {
	buf, err := ioutil.ReadFile(currentWorkingDirectoryFilePath("config.json"))
	if err != nil {
		return err
	}
	SepConfig = &sep.Config{}
	err = json.Unmarshal(buf, &SepConfig)
	if err != nil {
		return err
	}
	return nil
}

func loadClients() {
	buf, err := ioutil.ReadFile(currentWorkingDirectoryFilePath("clients.json"))
	if err != nil {
		log.Fatalln(err)
	}
	err = json.Unmarshal(buf, &Clients)
	if err != nil {
		log.Fatalln(err)
	}
}

func saveClients() error {
	buf, err := json.MarshalIndent(Clients, "", "\t")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(currentWorkingDirectoryFilePath("clients.json"), buf, 0644); err != nil {
		return err
	}
	return nil
}

func loadSafenetConfig() error {
	buf, err := ioutil.ReadFile(currentWorkingDirectoryFilePath("safenet.json"))
	if err != nil {
		return err
	}
	SafenetConfig = &safenet.Config{}
	err = json.Unmarshal(buf, &SafenetConfig)
	if err != nil {
		return err
	}
	return nil
}

func setSafenetConfig() error {
	cfg := &safenet.Config{
		LibPath:   "",
		UnlockPin: gen.Scan("Unesite PIN za digitalni token: "),
	}
	return saveSafeNetConfig(cfg)
}

func saveSafeNetConfig(cfg *safenet.Config) error {
	buf, err := json.MarshalIndent(cfg, "", "\t")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(currentWorkingDirectoryFilePath("safenet.json"), buf, 0644)
}

func saveSepConfig() error {

	buf, err := json.MarshalIndent(SepConfig, "", "\t")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(currentWorkingDirectoryFilePath("config.json"), buf, 0644)
}
