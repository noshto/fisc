package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
	WorkDir       = ""
)

func main() {
	WorkDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		showErrorAndExit(err)
	}
	os.Chdir(WorkDir)

	// create config.json
	if err := loadConfig(); err != nil {
		registerCompany()
	}
	// if it fails - exit
	if err := loadConfig(); err != nil {
		showErrorAndExit(err)
	}

	// make sure TCR registered, if not - register
	if SepConfig.TCR == nil {
		if err := registerTCR(); err != nil {
			showErrorAndExit(err)
		}
	}

	// load clients list, if fails - init with empty list
	if err := loadClients(); err != nil {
		Clients = &[]sep.Client{}
	}

	for {
		printUsage()

		stringValue := gen.Scan("Izaberite općiju: ")
		uint64Value, err := strconv.ParseUint(stringValue, 10, 64)
		if err != nil {
			fmt.Println("Pogrešna općija")
			continue
		}
		switch uint64Value {
		case 0:
			os.Exit(0)
		case 1:
			if err := registerInvoice(false); err != nil {
				showErrorAndExit(err)
			}
		case 2:
			if err := registerInvoice(true); err != nil {
				showErrorAndExit(err)
			}
		case 3:
			if err := registerCorrectiveInvoice(false); err != nil {
				showErrorAndExit(err)
			}
		case 4:
			if err := registerSummaryInvoice(false); err != nil {
				showErrorAndExit(err)
			}
		case 5:
			if err := generateIIC(); err != nil {
				showErrorAndExit(err)
			}
		case 6:
			if err := registerTCR(); err != nil {
				showErrorAndExit(err)
			}
		case 7:
			if err := registerClient(); err != nil {
				showErrorAndExit(err)
			}
		case 8:
			printCodes()
		case 9:
			fmt.Println()
			fmt.Println("---------------------------------------------------------------")
			fmt.Println("PREGLED IZVESTAJA ZA PERIOD")
			fmt.Println()
			fmt.Println("---------------------------------------------------------------")
			stringValue := gen.Scan("Datum od (u formati yyyy-MM-dd): ")
			from, err := time.Parse("2006-01-02", stringValue)
			if err != nil {
				showErrorAndExit(err)
			}
			stringValue = gen.Scan("Datum do (u formati yyyy-MM-dd): ")
			to, err := time.Parse("2006-01-02", stringValue)
			if err != nil {
				showErrorAndExit(err)
			}
			printSummary(from, to)
		}
	}
}

func printCodes() {
	fmt.Println("---------------------------------------------------------------")
	fmt.Println()
	fmt.Printf("PIB: %s\n", SepConfig.TIN)
	if SepConfig.TCR == nil {
		fmt.Printf("Kôd poslovne jedinice (prostora): %s\n", "NIJE REGISTRISANO")
	} else {
		fmt.Printf("Kôd poslovne jedinice (prostora): %s\n", SepConfig.TCR.BusinUnitCode)
	}
	if SepConfig.TCR == nil {
		fmt.Printf("Kôd elektronskog naplatnog uređaja: %s\n", "NIJE REGISTRISANO")
	} else {
		fmt.Printf("Kôd elektronskog naplatnog uređaja: %s\n", SepConfig.TCR.TCRCode)
	}
	if SepConfig.TCR == nil {
		fmt.Printf("Kôd softvera: %s\n", "NIJE REGISTRISANO")
	} else {
		fmt.Printf("Kôd softvera: %s\n", SepConfig.TCR.SoftCode)
	}
	if SepConfig.TCR == nil {
		fmt.Printf("Kôd održavaoca: %s\n", "NIJE REGISTRISANO")
	} else {
		fmt.Printf("Kôd održavaoca: %s\n", SepConfig.TCR.MaintainerCode)
	}
	fmt.Printf("Kôd operatera: %s\n", SepConfig.OperatorCode)

	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli: ")
}

// printUsage prints welcome message
func printUsage() {
	fmt.Println("---------------------------------------------------------------")
	fmt.Println()
	fmt.Println("Izaberite općiju:")
	fmt.Println("[1] REGISTRACIJA I FISKALIZACIJA RAČUNA")
	fmt.Println("[2] SKRACENA REGISTRACIJA I FISKALIZACIJA RAČUNA")
	fmt.Println("[3] REGISTRACIJA KOREKTIVNOG RAČUNA")
	fmt.Println("[4] REGISTRACIJA SUMARNOG RAČUNA")
	fmt.Println("[5] VERIFIKACIJA IKOF")
	fmt.Println("[6] REGISTRACIJA ENU")
	fmt.Println("[7] REGISTRACIJA KLIJENATA")
	fmt.Println("[8] PREGLED PODATAKA ENU")
	fmt.Println("[9] PREGLED IZVESTAJA ZA PERIOD")
	fmt.Println("[0] IZAĆI")
}

func showErrorAndExit(err error) {
	fmt.Println(err)
	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli: ")
	os.Exit(0)
}

func registerInvoice(simplified bool) error {
	if err := loadSafenetConfig(); err != nil {
		if err := setSafenetConfig(); err != nil {
			return err
		}
	}

	InternalOrdNum, err := gen.GenerateRegisterInvoiceRequest(&gen.Params{
		SepConfig:  SepConfig,
		Clients:    Clients,
		OutFile:    currentWorkingDirectoryFilePath("gen.xml"),
		Simplified: simplified,
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
	// check whether api succeeded
	buf, err := ioutil.ReadFile(currentWorkingDirectoryFilePath("reg.xml"))
	if err != nil {
		return err
	}
	RegisterInvoiceResponse := sep.RegisterInvoiceResponse{}
	if err := xml.Unmarshal(buf, &RegisterInvoiceResponse); err != nil {
		return err
	}
	if RegisterInvoiceResponse.Body.RegisterInvoiceResponse.FIC == "" {
		fmt.Println(RegisterInvoiceResponse.Body.Fault)
		return fmt.Errorf("%v", RegisterInvoiceResponse.Body.Fault)
	}
	fmt.Println("OK")

	fmt.Print("Generisanje PDF: ")
	if err := pdf.GeneratePDF(&pdf.Params{
		SepConfig:      SepConfig,
		Clients:        Clients,
		InternalInvNum: InternalOrdNum,
		ReqFile:        currentWorkingDirectoryFilePath("dsig.xml"),
		RespFile:       currentWorkingDirectoryFilePath("reg.xml"),
		OutFile:        currentWorkingDirectoryFilePath("inv.pdf"),
	}); err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Print("Čuvanje rezultata: ")
	folder, pdfFilePath, err := save(
		currentWorkingDirectoryFilePath("dsig.xml"),
		currentWorkingDirectoryFilePath("reg.xml"),
		currentWorkingDirectoryFilePath("inv.pdf"),
	)
	if err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Print("Čišćenje: ")
	if err := clean(
		currentWorkingDirectoryFilePath("gen.xml"),
		currentWorkingDirectoryFilePath("iic.xml"),
		currentWorkingDirectoryFilePath("dsig.xml"),
		currentWorkingDirectoryFilePath("reg.xml"),
		currentWorkingDirectoryFilePath("inv.pdf"),
	); err != nil {
		fmt.Println("NIJE USPEŠNO")
		return nil
	}
	fmt.Println("OK")

	fmt.Printf("Rezultate sačuvani u %s\n", folder)
	fmt.Printf("PDF fajl sačuvan u %s\n", pdfFilePath)

	return nil
}

func registerCorrectiveInvoice(simplified bool) error {
	if err := loadSafenetConfig(); err != nil {
		if err := setSafenetConfig(); err != nil {
			return err
		}
	}

	InternalOrdNum, err := gen.GenerateCorrectiveRegisterInvoiceRequest(&gen.Params{
		SepConfig:  SepConfig,
		Clients:    Clients,
		OutFile:    currentWorkingDirectoryFilePath("gen.xml"),
		Simplified: simplified,
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
	// check whether api succeeded
	buf, err := ioutil.ReadFile(currentWorkingDirectoryFilePath("reg.xml"))
	if err != nil {
		return err
	}
	RegisterInvoiceResponse := sep.RegisterInvoiceResponse{}
	if err := xml.Unmarshal(buf, &RegisterInvoiceResponse); err != nil {
		return err
	}
	if RegisterInvoiceResponse.Body.RegisterInvoiceResponse.FIC == "" {
		fmt.Println(RegisterInvoiceResponse.Body.Fault)
		return fmt.Errorf("%v", RegisterInvoiceResponse.Body.Fault)
	}
	fmt.Println("OK")

	fmt.Print("Generisanje PDF: ")
	if err := pdf.GeneratePDF(&pdf.Params{
		SepConfig:      SepConfig,
		Clients:        Clients,
		InternalInvNum: InternalOrdNum,
		ReqFile:        currentWorkingDirectoryFilePath("dsig.xml"),
		RespFile:       currentWorkingDirectoryFilePath("reg.xml"),
		OutFile:        currentWorkingDirectoryFilePath("inv.pdf"),
	}); err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Print("Čuvanje rezultata: ")
	folder, pdfFilePath, err := save(
		currentWorkingDirectoryFilePath("dsig.xml"),
		currentWorkingDirectoryFilePath("reg.xml"),
		currentWorkingDirectoryFilePath("inv.pdf"),
	)
	if err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Print("Čišćenje: ")
	if err := clean(
		currentWorkingDirectoryFilePath("gen.xml"),
		currentWorkingDirectoryFilePath("iic.xml"),
		currentWorkingDirectoryFilePath("dsig.xml"),
		currentWorkingDirectoryFilePath("reg.xml"),
		currentWorkingDirectoryFilePath("inv.pdf"),
	); err != nil {
		fmt.Println("NIJE USPEŠNO")
		return nil
	}
	fmt.Println("OK")

	fmt.Printf("Rezultate sačuvani u %s\n", folder)
	fmt.Printf("PDF fajl sačuvan u %s\n", pdfFilePath)

	return nil
}

func registerSummaryInvoice(simplified bool) error {
	if err := loadSafenetConfig(); err != nil {
		if err := setSafenetConfig(); err != nil {
			return err
		}
	}

	InternalOrdNum, err := gen.GenerateSummaryRegisterInvoiceRequest(&gen.Params{
		SepConfig:  SepConfig,
		Clients:    Clients,
		OutFile:    currentWorkingDirectoryFilePath("gen.xml"),
		Simplified: simplified,
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
	// check whether api succeeded
	buf, err := ioutil.ReadFile(currentWorkingDirectoryFilePath("reg.xml"))
	if err != nil {
		return err
	}
	RegisterInvoiceResponse := sep.RegisterInvoiceResponse{}
	if err := xml.Unmarshal(buf, &RegisterInvoiceResponse); err != nil {
		return err
	}
	if RegisterInvoiceResponse.Body.RegisterInvoiceResponse.FIC == "" {
		fmt.Println(RegisterInvoiceResponse.Body.Fault)
		return fmt.Errorf("%v", RegisterInvoiceResponse.Body.Fault)
	}
	fmt.Println("OK")

	fmt.Print("Generisanje PDF: ")
	if err := pdf.GeneratePDF(&pdf.Params{
		SepConfig:      SepConfig,
		Clients:        Clients,
		InternalInvNum: InternalOrdNum,
		ReqFile:        currentWorkingDirectoryFilePath("dsig.xml"),
		RespFile:       currentWorkingDirectoryFilePath("reg.xml"),
		OutFile:        currentWorkingDirectoryFilePath("inv.pdf"),
	}); err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Print("Čuvanje rezultata: ")
	folder, pdfFilePath, err := save(
		currentWorkingDirectoryFilePath("dsig.xml"),
		currentWorkingDirectoryFilePath("reg.xml"),
		currentWorkingDirectoryFilePath("inv.pdf"),
	)
	if err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Print("Čišćenje: ")
	if err := clean(
		currentWorkingDirectoryFilePath("gen.xml"),
		currentWorkingDirectoryFilePath("iic.xml"),
		currentWorkingDirectoryFilePath("dsig.xml"),
		currentWorkingDirectoryFilePath("reg.xml"),
		currentWorkingDirectoryFilePath("inv.pdf"),
	); err != nil {
		fmt.Println("NIJE USPEŠNO")
		return nil
	}
	fmt.Println("OK")

	fmt.Printf("Rezultate sačuvani u %s\n", folder)
	fmt.Printf("PDF fajl sačuvan u %s\n", pdfFilePath)

	return nil
}

func generateIIC() error {
	fmt.Println()
	fmt.Println("---------------------------------------------------------------")
	fmt.Println("VERIFIKACIJA IKOF")

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
	return filepath.Join(WorkDir, fileName)
}

func registerTCR() error {
	if err := gen.GenerateRegisterTCRRequest(&gen.Params{
		SepConfig: SepConfig,
		OutFile:   currentWorkingDirectoryFilePath("tcr.xml"),
	}); err != nil {
		return err
	}

	if err := loadSafenetConfig(); err != nil {
		if err := setSafenetConfig(); err != nil {
			return err
		}
	}

	fmt.Print("Generisanje DSIG: ")
	if err := dsig.Sign(&dsig.Params{
		SepConfig:     SepConfig,
		SafenetConfig: SafenetConfig,
		InFile:        currentWorkingDirectoryFilePath("tcr.xml"),
		OutFile:       currentWorkingDirectoryFilePath("tcr.dsig.xml"),
	}); err != nil {
		return err
	}
	fmt.Println("OK")

	fmt.Print("Registrovanje: ")
	if err := reg.Register(&reg.Params{
		SafenetConfig: SafenetConfig,
		SepConfig:     SepConfig,
		InFile:        currentWorkingDirectoryFilePath("tcr.dsig.xml"),
		OutFile:       currentWorkingDirectoryFilePath("tcr.reg.xml"),
	}); err != nil {
		return err
	}
	// check whether api succeeded
	buf, err := ioutil.ReadFile(currentWorkingDirectoryFilePath("tcr.reg.xml"))
	if err != nil {
		return err
	}
	RegisterTCRResponse := sep.RegisterTCRResponse{}
	if err := xml.Unmarshal(buf, &RegisterTCRResponse); err != nil {
		return err
	}
	if RegisterTCRResponse.Body.RegisterTCRResponse.TCRCode == "" {
		fmt.Println(RegisterTCRResponse.Body.Fault)
		return fmt.Errorf("%v", RegisterTCRResponse.Body.Fault)
	}
	fmt.Println("OK")

	fmt.Print("Čuvanje rezultata: ")
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
	fmt.Println("OK")

	fmt.Print("Čišćenje: ")
	if err := clean(
		currentWorkingDirectoryFilePath("tcr.xml"),
		currentWorkingDirectoryFilePath("tcr.dsig.xml"),
		currentWorkingDirectoryFilePath("tcr.reg.xml"),
	); err != nil {
		fmt.Println("NIJE USPEŠNO")
		return nil
	}

	fmt.Println("Detalji ENU su uspešno registrovani i sačuvani")
	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli u glavno meni: ")
	return nil
}

// generateClient asks user to fill in new client details
func generateClient() *sep.Client {
	fmt.Println()
	fmt.Println("---------------------------------------------------------------")
	fmt.Println("REGISTRACIJA KLIJENATA")
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
	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli u glavno meni: ")
	return nil
}

func registerCompany() error {
	fmt.Println("---------------------------------------------------------------")
	fmt.Println("Welcome to FISC - simple util that saves you from frustrating process of invoices fiscalization!")
	fmt.Println("---------------------------------------------------------------")
	fmt.Println("This app is intented to help you with generation of an invoice request that meets efi.tax.gov.me fiscalization service requirements.")
	fmt.Println("You will be asked to answer the minimal list of questions sufficient for invoice fiscalization.")
	fmt.Println()
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
		return err
	}
	err = ioutil.WriteFile(currentWorkingDirectoryFilePath("config.json"), buf, 0644)
	if err != nil {
		return err
	}
	fmt.Println("Detalji su uspešno sačuvani")
	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli u glavno meni: ")
	return nil
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

func loadClients() error {
	buf, err := ioutil.ReadFile(currentWorkingDirectoryFilePath("clients.json"))
	if err != nil {
		return err
	}
	err = json.Unmarshal(buf, &Clients)
	if err != nil {
		return err
	}
	return nil
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
	SafenetConfig = &safenet.Config{
		LibPath:   "",
		UnlockPin: gen.Scan("Unesite PIN za digitalni token: "),
	}
	return saveSafeNetConfig(SafenetConfig)
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

func save(requestFilePath, responseFilePath, pdfFilePath string) (string, string, error) {

	// generate output folder, ./records/<DATE>
	workDir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		return "", "", err
	}

	recordsDir := filepath.Join(workDir, "records")
	currentDayDir := filepath.Join(recordsDir, time.Now().Format("2006-01-02"))

	if _, err := os.Stat(currentDayDir); os.IsNotExist(err) {
		if err := os.MkdirAll(currentDayDir, 0755); err != nil {
			return "", "", err
		}
	}

	// save RegisterInvoiceRequest
	doc := etree.NewDocument()
	if err := doc.ReadFromFile(requestFilePath); err != nil {
		return "", "", err
	}
	reqFileName, err := requestFileName(doc)
	if err != nil {
		return "", "", err
	}
	reqFilePath := filepath.Join(currentDayDir, reqFileName)
	elem := doc.FindElement("//RegisterInvoiceRequest")
	if elem == nil {
		return "", "", fmt.Errorf("invalid xml, RegisterInvoiceRequest")
	}
	reqDoc := etree.NewDocument()
	reqDoc.SetRoot(elem.Copy())
	reqDoc.IndentTabs()
	reqDoc.Root().SetTail("")
	if err := reqDoc.WriteToFile(reqFilePath); err != nil {
		return "", "", err
	}

	// save RegisterInvoiceResponse
	respFileName, err := responseFileName(doc)
	if err != nil {
		return "", "", err
	}
	respFilePath := filepath.Join(currentDayDir, respFileName)
	doc = etree.NewDocument()
	if err := doc.ReadFromFile(responseFilePath); err != nil {
		return "", "", err
	}
	elem = doc.FindElement("//RegisterInvoiceResponse")
	if elem == nil {
		return "", "", fmt.Errorf("invalid xml, RegisterInvoiceResponse")
	}
	reqDoc = etree.NewDocument()
	reqDoc.SetRoot(elem.Copy())
	reqDoc.IndentTabs()
	reqDoc.Root().SetTail("")
	if err := reqDoc.WriteToFile(respFilePath); err != nil {
		return "", "", err
	}

	// save pdf
	buf, err := ioutil.ReadFile(pdfFilePath)
	if err != nil {
		return "", "", err
	}
	extension := filepath.Ext(reqFileName)
	pdfFileName := strings.Join([]string{reqFileName[0 : len(reqFileName)-len(extension)], "pdf"}, ".")
	invoiceFilePath := filepath.Join(currentDayDir, pdfFileName)
	if err := ioutil.WriteFile(invoiceFilePath, buf, 0644); err != nil {
		return "", "", err
	}
	invoiceFilePath = currentWorkingDirectoryFilePath(pdfFileName)
	if err := ioutil.WriteFile(invoiceFilePath, buf, 0644); err != nil {
		return "", "", err
	}
	return currentDayDir, invoiceFilePath, nil
}

func requestFileName(doc *etree.Document) (string, error) {

	invoice := doc.FindElement("//RegisterInvoiceRequest").FindElement("Invoice")
	if invoice == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no Invoice"))
		os.Exit(1)
	}

	tmp := invoice.SelectAttr("IssueDateTime")
	if tmp == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no IssueDateTime"))
		os.Exit(1)
	}
	IssueDateTime, err := time.Parse(time.RFC3339, tmp.Value)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	tmp = invoice.SelectAttr("TCRCode")
	if tmp == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no TCRCode"))
		os.Exit(1)
	}
	TCRCode := tmp.Value

	tmp = invoice.SelectAttr("IIC")
	if tmp == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no IIC"))
		os.Exit(1)
	}
	IIC := tmp.Value

	fileName := strings.Join([]string{IssueDateTime.Format("20060102150405"), TCRCode, IIC, "request.xml"}, "_")

	return fileName, nil
}

func responseFileName(doc *etree.Document) (string, error) {

	invoice := doc.FindElement("//Invoice")
	if invoice == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no Invoice"))
		os.Exit(1)
	}

	tmp := invoice.SelectAttr("IssueDateTime")
	if tmp == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no IssueDateTime"))
		os.Exit(1)
	}
	IssueDateTime, err := time.Parse(time.RFC3339, tmp.Value)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	tmp = invoice.SelectAttr("TCRCode")
	if tmp == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no TCRCode"))
		os.Exit(1)
	}
	TCRCode := tmp.Value

	tmp = invoice.SelectAttr("IIC")
	if tmp == nil {
		fmt.Fprintln(os.Stderr, errors.New("Invalid XML, no IIC"))
		os.Exit(1)
	}
	IIC := tmp.Value

	fileName := strings.Join([]string{IssueDateTime.Format("20060102150405"), TCRCode, IIC, "response.xml"}, "_")

	return fileName, nil
}

func clean(files ...string) error {
	for _, it := range files {
		if err := os.Remove(it); err != nil {
			return err
		}
	}
	return nil
}

func printSummary(startDate, endDate time.Time) {

	// enumerate all folders from startDate to endDate
	dates := []string{}
	var it time.Time = startDate
	for {
		dates = append(dates, it.Format("2006-01-02"))
		it = it.AddDate(0, 0, 1)
		if it.After(endDate) {
			break
		}
	}

	recordsDir := filepath.Join(WorkDir, "records")

	Num := 0
	PBWoR := sep.Amount(0)
	PBR := sep.Amount(0)
	R := sep.Amount(0)
	VA := sep.Amount(0)
	Total := sep.Amount(0)

	// gather requests from all selected folders
	requests := []sep.RegisterInvoiceRequest{}
	for _, folder := range dates {
		// check if folder exists
		dateDir := filepath.Join(recordsDir, folder)
		if _, err := os.Stat(dateDir); os.IsNotExist(err) {
			continue
		}

		files, err := ioutil.ReadDir(dateDir)
		if err != nil {
			fmt.Println(err.Error())
			continue
		}

		// read each _request.xml file and retrieve content
		for _, fi := range files {

			if strings.Contains(fi.Name(), "_request.xml") {
				filePath := filepath.Join(dateDir, fi.Name())

				request := sep.RegisterInvoiceRequest{}
				buf, err := ioutil.ReadFile(filePath)
				if err != nil {
					fmt.Println(err.Error())
				}
				if err := xml.Unmarshal(buf, &request); err != nil {
					fmt.Println(err.Error())
				}
				requests = append(requests, request)
			}
		}

		Num += len(requests)

		for _, req := range requests {
			for _, i := range *req.Invoice.Items {
				PBWoR += i.UPB * i.Q

				upbr := i.UPB - i.UPB*(i.R/100)
				PBR += upbr * i.Q

				R += i.UPB*i.Q - upbr*i.Q

				uva := upbr * (i.VR / 100)
				VA += uva * i.Q
			}
			Total += req.Invoice.TotPrice
		}

		requests = []sep.RegisterInvoiceRequest{}
	}

	fmt.Println("---------------------------------------------------------------")
	fmt.Printf("Koliko ukupno faktura: %d\n", Num)
	fmt.Printf("Koliko osnovica prije rabata: %.02f\n", float64(PBWoR))
	fmt.Printf("Koliko rabat: %.02f\n", float64(R))
	fmt.Printf("Koliko osnovica posle rabata: %.02f\n", float64(PBR))
	fmt.Printf("Koliko PDV: %.02f\n", float64(VA))
	fmt.Printf("Koliko ukupno sa PDV: %.02f\n", float64(Total))
	fmt.Println("---------------------------------------------------------------")

	fileName := strings.Join([]string{"izveštaj", startDate.Format("2006-01-02"), endDate.Format("2006-01-02")}, "_")
	fileName = strings.Join([]string{fileName, "pdf"}, ".")
	filePath := currentWorkingDirectoryFilePath(fileName)
	if err := pdf.GenerateExempt(
		SepConfig,
		startDate,
		endDate,
		Num,
		float64(PBWoR),
		float64(R),
		float64(PBR),
		float64(VA),
		float64(Total),
		filePath,
	); err != nil {
		showErrorAndExit(err)
	}

	fmt.Println()
	fmt.Printf("Izveštaj sačuvan: %s\n", filePath)

	fmt.Println()
	_ = gen.Scan("Pritisnite bilo koji taster da biste izašli: ")
}
