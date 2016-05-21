package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"regexp"
	"strings"
)

var rootDir string
var buildDir = "build"
var sourcesDir = "src"
var resultDir = "result"
var sourcesPath = path.Join(buildDir, sourcesDir)
var arch_iphoneos = [3]string{"armv7", "armv7s", "arm64"}
var arch_simulator = [2]string{"i386", "x86_64"}

var pjsipUrl = "http://www.pjsip.org/release/2.5/pjproject-2.5.tar.bz2"
var pjsipFile = "pjsip.tar.bz2"
var pjsipPath = path.Join(buildDir, pjsipFile)
var resultPath = path.Join(buildDir, resultDir)

func lipo() {
	os.Chdir(rootDir)
	libPath := path.Join(resultPath, "lib")
	grouped := map[string][]string{}

	files, err := ioutil.ReadDir(libPath)

	if err != nil {
		log.Println(err)
		return
	}

	for _, file := range files {
		name := file.Name()

		if match, _ := regexp.MatchString("^lib", name); !match {
			log.Println("skip", name)
			continue
		}

		name = regexp.MustCompile("-armv7|-armv7s|-arm64|-i386|-x86_64").Split(name, -1)[0]

		grouped[name] = append(grouped[name], path.Join(libPath, file.Name()))
	}

	for libName, array := range grouped {
		resultLib := path.Join(libPath, fmt.Sprintf("%v.a", libName))

		os.Remove(resultLib)
		args := "-create -output " + resultLib + " " + strings.Join(array, " ")

		cmd := exec.Command("lipo", strings.Fields(args)...)

		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}

		for _, file := range array {
			if err := os.Remove(file); err != nil {
				log.Println(err)
			}
		}
	}
}

func download() {
	log.Println("-- DOWNLOAD STAGE --")
	defer log.Println("-- DOWNLOAD STAGE END --")

	if _, err := os.Stat(pjsipPath); err == nil {
		log.Println("pjsip already downloaded")
		return
	}

	output, err := os.Create(pjsipPath)

	if err != nil {
		log.Printf("Error while creating pjsip file - %v", err)
		return
	}

	defer output.Close()

	response, err := http.Get(pjsipUrl)

	if err != nil {
		log.Printf("Error while downloading pjsip - %v", err)
		return
	}

	defer response.Body.Close()

	n, err := io.Copy(output, response.Body)
	if err != nil {
		log.Printf("Error while downloading pjsip - %v", err)
		return
	}

	log.Println(n, "bytes copied to", output.Name())
}

func extract() {
	log.Println("-- EXTRACT STAGE --")
	defer log.Println("-- EXTRACT STAGE END --")

	if err := os.RemoveAll(sourcesPath); err != nil {
		log.Println(err)
		return
	}

	if err := os.Mkdir(sourcesPath, 0755); err != nil {
		log.Println(err)
		return
	}

	command := exec.Command("tar", "-xf", pjsipPath, "-C", sourcesPath, "--strip-components", "1")

	out, err := command.CombinedOutput()

	if err != nil {
		log.Println(err, string(out[:]))
		return
	}

	log.Println("extracted to", sourcesPath)
}

func configure() {
	log.Println("-- CONFIGURE STAGE --")
	defer log.Println("-- CONFIGURE STAGE END --")

	configSitePath := path.Join(sourcesPath, "pjlib", "include", "pj", "config_site.h")

	file, err := os.Create(configSitePath)

	if err != nil {
		log.Println(err)
		return
	}

	file.WriteString("#define PJ_CONFIG_IPHONE 1\n")
	file.WriteString("#include <pj/config_site_sample.h>\n")

	if err := os.RemoveAll(resultPath); err != nil {
		log.Println(err)
		return
	}

	if err := os.Mkdir(resultPath, 0755); err != nil {
		log.Println(err)
		return
	}
}

func buildArm() {
	log.Println("-- BUILD ARM STAGE --")
	defer log.Println("-- BUILD ARM STAGE END --")

	os.Chdir(sourcesPath)

	for _, v := range arch_iphoneos {
		log.Println("building for", v)

		env := os.Environ()
		env = append(env, "CFLAGS=-miphoneos-version-min=7.0")
		env = append(env, "LDFLAGS=-miphoneos-version-min=7.0")
		env = append(env, fmt.Sprintf("ARCH=-arch %v", v))

		log.Println("configure-iphone")
		cmd := exec.Command("./configure-iphone", fmt.Sprintf("--prefix=%v", path.Join(rootDir, resultPath)))
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}

		log.Println("make dep")
		cmd = exec.Command("make", "dep")
		cmd.Env = env
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}

		log.Println("make")
		cmd = exec.Command("make")
		cmd.Env = env
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}

		log.Println("make install")
		cmd = exec.Command("make", "install")
		cmd.Env = env
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}

		log.Println("make clean")
		cmd = exec.Command("make", "clean")
		cmd.Env = env
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}
	}
}

func buildX86() {
	log.Println("-- BUILD X86 STAGE --")
	defer log.Println("-- BUILD X86 STAGE END --")

	os.Chdir(sourcesPath)

	for _, v := range arch_simulator {
		log.Println("building for", v)

		env := os.Environ()
		env = append(env, "CFLAGS=-O2 -m32 -mios-simulator-version-min=7.0")
		env = append(env, "LDFLAGS=-O2 -m32 -mios-simulator-version-min=7.0")
		env = append(env, "DEVPATH=/Applications/Xcode.app/Contents/Developer/Platforms/iPhoneSimulator.platform/Developer")
		env = append(env, fmt.Sprintf("ARCH=-arch %v", v))

		log.Println("configure-iphone")
		cmd := exec.Command("./configure-iphone", fmt.Sprintf("--prefix=%v", path.Join(rootDir, resultPath)))
		cmd.Env = env
		out, err := cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}

		log.Println("make dep")
		cmd = exec.Command("make", "dep")
		cmd.Env = env
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}

		log.Println("make")
		cmd = exec.Command("make")
		cmd.Env = env
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}

		log.Println("make install")
		cmd = exec.Command("make", "install")
		cmd.Env = env
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}

		log.Println("make clean")
		cmd = exec.Command("make", "clean")
		cmd.Env = env
		out, err = cmd.CombinedOutput()
		if err != nil {
			log.Println(err, string(out[:]))
			return
		}
	}
}

func main() {
	var err error

	rootDir, err = os.Getwd()

	if err != nil {
		return
	}

	download()
	extract()
	configure()
	buildArm()
	buildX86()
	lipo()
}
