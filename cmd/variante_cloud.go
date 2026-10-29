//go:build aws && azure && gcp

package cmd

// varianteDoPacote é o nome do pacote e do binário desta variante no release: o yby-cloud
// (task build:cloud e o GoReleaser) é compilado com as três tags de nuvem.
const varianteDoPacote = "yby-cloud"
