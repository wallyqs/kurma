// Copyright 2015 Apcera Inc. All rights reserved.

package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/apcera/kurma/pkg/devices"
)

type bootPart int

const (
	IMGA = bootPart(iota)
	IMGB
)

func vmlinuzImg(img bootPart) string {
	switch img {
	case IMGA:
		return "kurmaos/vmlinuz-a"
	case IMGB:
		return "kurmaos/vmlinuz-b"
	}
	panic("logic error")
}

func initrdImg(img bootPart) string {
	switch img {
	case IMGA:
		return "/kurmaos/initrd-a"
	case IMGB:
		return "/kurmaos/initrd-b"
	}
	panic("logic error")
}

func main() {
	// determine which partition we've currently booted with
	bootedDev, err := getCurrentBootedPartition()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// choose which image we will be applying the update to
	img, newbootdev, err := determineDestinationImage(bootedDev)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// write the kernel images
	fmt.Fprintln(os.Stderr, "Writing new kernel/initrd images...")
	if err := writeKernelImage(img); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// update the GPT priorities/tries
	if err := updateGptPriority(newbootdev); err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err.Error())
		os.Exit(1)
	}

	// boot the new kernel image
	if err := bootNewKernel(img, newbootdev); err != nil {
		fmt.Fprintf(os.Stderr, "kexec failed: %s\n", err.Error())
		fmt.Fprintf(os.Stderr, "Attempting full reboot...\n")
		if err := fullReboot(); err != nil {
			fmt.Fprintf(os.Stderr, "reboot failed: %s\n", err.Error())
			os.Exit(1)
		}
	}
}

func getCurrentBootedPartition() (string, error) {
	bootedStr, err := getBootedPartitionFromCmdline()
	if err != nil {
		return "", err
	}

	dev := devices.ResolveDevice(bootedStr)
	if dev == "" {
		return "", fmt.Errorf("Failed to resolve booted partition of %q", bootedStr)
	}
	return dev, nil
}

func determineDestinationImage(bootedDev string) (bootPart, string, error) {
	adev := devices.ResolveDevice("PARTLABEL=KURMA-A")
	if adev == "" {
		return bootPart(0), "", fmt.Errorf("Failed to locate KURMA-A boot partition")
	}
	bdev := devices.ResolveDevice("PARTLABEL=KURMA-B")
	if bdev == "" {
		return bootPart(0), "", fmt.Errorf("Failed to locate KURMA-B boot partition")
	}

	// if the current booted device matches the one we resolved for the image,
	// then we return the opposite one as the destination.
	switch bootedDev {
	case adev:
		return IMGB, bdev, nil
	case bdev:
		return IMGA, adev, nil
	default:
		return bootPart(0), "", fmt.Errorf("Failed to match booted device of %q to a known partition", bootedDev)
	}
}

const mountPath = "/mnt"

func writeKernelImage(dest bootPart) error {
	// locate the partition with the kernels
	efiDevice := devices.ResolveDevice("LABEL=EFI-SYSTEM")
	if efiDevice == "" {
		return fmt.Errorf("Failed to locate the EFI-SYSTEM partition")
	}

	// mount the efi-system partition
	os.Mkdir(mountPath, os.FileMode(0755))
	if err := syscall.Mount(efiDevice, mountPath, "vfat", 0, ""); err != nil {
		return fmt.Errorf("Failed to mount the EFI-SYSTEM partition: %v", err)
	}
	defer syscall.Unmount(mountPath, 0)

	// open the new kernel/inird in the image
	kernFile, err := os.Open("bzImage")
	if err != nil {
		return fmt.Errorf("Failed to open new bzImage: %v", err)
	}
	defer kernFile.Close()
	initFile, err := os.Open("initrd")
	if err != nil {
		return fmt.Errorf("Failed to open new initrd: %v", err)
	}
	defer initFile.Close()

	// open the destination kernel/inird files
	flags := os.O_CREATE | os.O_TRUNC | os.O_WRONLY
	newKernFile, err := os.OpenFile(filepath.Join(mountPath, vmlinuzImg(dest)), flags, os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("Failed to open destination bzImage: %v", err)
	}
	defer newKernFile.Close()
	newInitFile, err := os.OpenFile(filepath.Join(mountPath, initrdImg(dest)), flags, os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("Failed to open destination initrd: %v", err)
	}
	defer newInitFile.Close()

	// write the files
	if _, err := io.Copy(newKernFile, kernFile); err != nil {
		return fmt.Errorf("Failed to write kernel: %v", err)
	}
	if _, err := io.Copy(newInitFile, initFile); err != nil {
		return fmt.Errorf("Failed to write initrd: %v", err)
	}
	return nil
}

func updateGptPriority(newbootdev string) error {
	rawdev := devices.GetRawDevice(newbootdev)
	if rawdev == "" {
		return fmt.Errorf("Failed to find raw device for %q", newbootdev)
	}
	partnum := devices.GetPartitionNumber(newbootdev)
	if partnum == "" {
		return fmt.Errorf("Failed to find partition number for %q", newbootdev)
	}

	// Clear the success flag, mark the partition with 1 try, then prioritize it
	out, err := exec.Command("/cgpt", "add", rawdev, "-i", partnum, "-S0", "-T1").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to configure partition tries [device: %q, partition: %q]: %s", rawdev, partnum, string(out))
	}
	out, err = exec.Command("/cgpt", "prioritize", rawdev, "-i", partnum).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to prioritize boot device [device: %q, partition: %q]: %s", rawdev, partnum, string(out))
	}
	return nil
}

func bootNewKernel(img bootPart, newdev string) error {
	// locate the partition with the kernels
	efiDevice := devices.ResolveDevice("LABEL=EFI-SYSTEM")
	if efiDevice == "" {
		return fmt.Errorf("Failed to resolve EFI-SYSTEM partition")
	}

	// mount the efi-system partition
	os.Mkdir(mountPath, os.FileMode(0755))
	if err := syscall.Mount(efiDevice, mountPath, "vfat", 0, ""); err != nil {
		return fmt.Errorf("Failed to mount EFI-SYSTEM: %v", err)
	}

	kernel := filepath.Join(mountPath, vmlinuzImg(img))
	initrd := filepath.Join(mountPath, initrdImg(img))
	newcmdline, err := getNewCmdline(img, newdev)
	if err != nil {
		return fmt.Errorf("Failed to generate command line: %v", err)
	}

	b, err := exec.Command("/kexec", "--load", kernel, "--initrd", initrd, "--command-line", newcmdline).CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to set kexec settings: %s", string(b))
	}
	b, err = exec.Command("/kexec", "--exec").CombinedOutput()
	if err != nil {
		return fmt.Errorf("Failed to kexec: %s", string(b))
	}
	return nil
}

func getBootedPartitionFromCmdline() (string, error) {
	b, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return "", err
	}
	str := strings.Trim(string(b), "\n")
	dev := parseCmdline(str)
	if dev == "" {
		return "", fmt.Errorf("Failed to located booted device specification")
	}
	return dev, nil
}

func parseCmdline(cmdline string) string {
	for _, part := range strings.Split(cmdline, " ") {
		if !strings.HasPrefix(part, "kurma.booted=") {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		return kv[1]
	}
	return ""
}

func getNewCmdline(img bootPart, bootedDev string) (string, error) {
	b, err := ioutil.ReadFile("/proc/cmdline")
	if err != nil {
		return "", err
	}
	str := strings.Trim(string(b), "\n")
	return generateNewCmdline(str, img, bootedDev), nil
}

func generateNewCmdline(cmdline string, img bootPart, bootedDev string) string {
	parts := strings.Split(cmdline, " ")
	for i, part := range parts {
		// Update BOOT_IMAGE which references the kernel that is booted.
		if strings.HasPrefix(part, "BOOT_IMAGE=") {
			parts[i] = fmt.Sprintf("BOOT_IMAGE=/%s", vmlinuzImg(img))
			continue
		}

		// Update the partition handler we are booting into.
		if strings.HasPrefix(part, "kurma.booted=") {
			parts[i] = fmt.Sprintf("kurma.booted=%s", bootedDev)
		}
	}

	return strings.Join(parts, " ")
}

func fullReboot() error {
	f, err := os.OpenFile("/host/proc/sysrq-trigger", os.O_WRONLY, os.FileMode(0644))
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write([]byte{'b'})
	return err
}
