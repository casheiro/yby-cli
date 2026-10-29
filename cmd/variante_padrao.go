//go:build !(aws && azure && gcp)

package cmd

// varianteDoPacote é o nome do pacote e do binário desta variante no release: o yby
// padrão, sem os SDKs de nuvem.
const varianteDoPacote = "yby"
