package main

// Extraction des définitions protobuf embarquées dans StarCitizen.exe :
// l'outil reste compatible avec la version installée, même après un patch.

import (
	"fmt"
	"os"
	"regexp"

	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

var protoName = regexp.MustCompile(`sc/game/telemetry/v1/[A-Za-z0-9_/]+\.proto`)

func extractDescriptors(exe string) (*descriptorpb.FileDescriptorSet, error) {
	data, err := os.ReadFile(exe)
	if err != nil {
		return nil, err
	}
	found := map[string]*descriptorpb.FileDescriptorProto{}
	for _, loc := range protoName.FindAllIndex(data, -1) {
		s, name := loc[0], string(data[loc[0]:loc[1]])
		for _, back := range []int{2, 3} {
			st := s - back
			if st < 0 || data[st] != 0x0a {
				continue
			}
			n, k := protowire.ConsumeVarint(data[st+1:])
			if k < 0 || st+1+k != s || int(n) != len(name) {
				continue
			}
			// avance champ par champ tant que ça ressemble à un FileDescriptorProto
			i, last := st, st
			for i < len(data) {
				num, typ, k := protowire.ConsumeTag(data[i:])
				if k < 0 || num < 1 || num > 15 || (typ != protowire.VarintType && typ != protowire.BytesType) {
					break
				}
				m := protowire.ConsumeFieldValue(num, typ, data[i+k:])
				if m < 0 {
					break
				}
				i += k + m
				last = i
			}
			for cut := last; cut > st && cut > last-64; cut-- {
				fd := &descriptorpb.FileDescriptorProto{}
				if proto.Unmarshal(data[st:cut], fd) == nil && fd.GetName() == name {
					if prev, ok := found[name]; !ok || len(fd.MessageType) > len(prev.MessageType) {
						found[name] = fd
					}
					break
				}
			}
		}
	}
	if len(found) == 0 {
		return nil, fmt.Errorf("aucune définition de télémétrie trouvée dans %s", exe)
	}
	set := &descriptorpb.FileDescriptorSet{}
	for _, fd := range found {
		set.File = append(set.File, fd)
	}
	return set, nil
}
