package response

type MintResult struct {
	ImageCid    string `json:"imageCid"`
	MetadataCid string `json:"metadataCid"`
	TokenUrl    string `json:"tokenUrl"`    // "ipfs://" + MetadataCid,
	MetadataUrl string `json:"metadataUrl"` //"https://" + *** + "/ipfs/" + imageCID,
	ImageUrl    string `json:"imageUrl"`    //"https://" + *** + "/ipfs/" + metadataCID,
}
