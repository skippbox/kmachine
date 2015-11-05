package kubernetes

import (
    "crypto/rand"
    "fmt"
	"path"
)

type KubernetesOptions struct {
    K8SToken        string
    K8SCertPath     string
    K8SAPICert      string
    K8SAPIKey       string
    K8SProxyCert    string
    K8SProxyKey     string
    K8SAdminCert    string
    K8SAdminKey     string
}

/* Random password generator for the token */
func GenerateRandomToken(size int) string {
    bytestr := make([]byte, 1)
    encstr := make([]byte, size)
    i := 0
 
    for i < size {
        _, err := rand.Read(bytestr)

        if err != nil {
            fmt.Printf("Error: ", err)
            return ""
        }

        if ((bytestr[0] > 0x2F) && (bytestr[0] < 0x3A)) ||
            ((bytestr[0] > 0x40) && (bytestr[0] < 0x5B)) ||
            ((bytestr[0] > 0x60) && (bytestr[0] < 0x7B)) {
            encstr[i] = bytestr[0]
            i = i+1
        }
    }

    return fmt.Sprintf("%s", encstr)
}

func GenOrg(name string, app string) (string) {
	return name + "-" + app
}

func GetBase(dir string, name string, app string) (string) {
	base := GenOrg(name, app) + "_"
	return path.Join(dir, base)
}
