package connectors

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path"

	"github.com/IBM/ubiquity/resources"
	"github.com/IBM/ubiquity/utils"
)

type spectrum_rest struct {
	logger     *log.Logger
	httpClient *http.Client
	endpoint   string
}

func NewSpectrumRest(logger *log.Logger, restConfig resources.RestConfig) (SpectrumScaleConnector, error) {
	endpoint := restConfig.Endpoint
	return &spectrum_rest{logger: logger, httpClient: &http.Client{}, endpoint: endpoint}, nil
}

func NewSpectrumRestWithClient(logger *log.Logger, restConfig resources.RestConfig, client *http.Client) (SpectrumScaleConnector, error) {
	endpoint := restConfig.Endpoint
	return &spectrum_rest{logger: logger, httpClient: client, endpoint: endpoint}, nil
}

func (s *spectrum_rest) GetClusterId() (string, error) {
	getClusterURL := utils.FormatURL(s.endpoint, "scalemgmt/v1/cluster")
	getClusterResponse := GetClusterResponse{}
	cidResponse, err := s.doHTTP(getClusterURL, "GET", getClusterResponse, nil)
	if err != nil {
		s.logger.Printf("error in executing remote call: %v", err)
		return "", err
	}

	getClusterResponse = cidResponse.(GetClusterResponse)

	return getClusterResponse.Cluster.ClusterSummary.ClusterID, nil
}

func (s *spectrum_rest) IsFilesystemMounted(filesystemName string) (bool, error) {
	//TODO check that this is the right url ?
	getNodesURL := utils.FormatURL(s.endpoint, "scalemgmt/v1/nodes")

	getNodesResponse := GetNodesResponse{}
	nodesResponse, err := s.doHTTP(getNodesURL, "GET", getNodesResponse, nil)
	if err != nil {
		s.logger.Printf("error in executing remote call: %v", err)
		return false, err
	}

	getNodesResponse = nodesResponse.(GetNodesResponse)

	currentNode, _ := os.Hostname()
	s.logger.Printf("spectrum rest Client: node name: %s\n", currentNode)
	for _, node := range getNodesResponse.Nodes {
		if node.NodeName == currentNode {
			return true, nil
		}
	}

	return false, nil
}

func (s *spectrum_rest) MountFileSystem(filesystemName string) error {
	fmt.Printf("This method is not yet implemented")
	return nil
}

func (s *spectrum_rest) ListFilesystems() ([]string, error) {
	listFilesystemsURL := utils.FormatURL(s.endpoint, "scalemgmt/v1/filesystems")
	getFilesystemResponse := GetFilesystemResponse{}
	fsResponse, err := s.doHTTP(listFilesystemsURL, "GET", getFilesystemResponse, nil)
	if err != nil {
		s.logger.Printf("error in executing remote call: %v", err)
		return nil, err
	}

	getFilesystemResponse = fsResponse.(GetFilesystemResponse)

	fsNumber := len(getFilesystemResponse.FileSystems)
	filesystems := make([]string, fsNumber)
	for i := 0; i < fsNumber; i++ {
		filesystems[i] = getFilesystemResponse.FileSystems[i].FilesystemName
	}
	return filesystems, nil
}

func (s *spectrum_rest) GetFilesystemMountpoint(filesystemName string) (string, error) {
	getFilesystemURL := utils.FormatURL(s.endpoint, fmt.Sprintf("scalemgmt/v1/filesystems/%s", filesystemName))
	getFilesystemResponse := GetFilesystemResponse{}

	fsResponse, err := s.doHTTP(getFilesystemURL, "GET", getFilesystemResponse, nil)
	if err != nil {
		s.logger.Printf("error in executing remote call: %v", err)
		return "", err
	}

	getFilesystemResponse = fsResponse.(GetFilesystemResponse)
	return getFilesystemResponse.FileSystems[0].DefaultMountPoint, nil
}

func (s *spectrum_rest) CreateFileset(filesystemName string, filesetName string, opts map[string]interface{}) error {
	filesetConfig := FilesetConfig{}
	filesetConfig.Comment = "fileset for container volume"
	filesetConfig.FilesetName = filesetName
	filesetConfig.FilesystemName = filesystemName

	filesetType, filesetTypeSpecified := opts[USER_SPECIFIED_FILESET_TYPE]
	inodeLimit, inodeLimitSpecified := opts[USER_SPECIFIED_INODE_LIMIT]

	if filesetTypeSpecified && filesetType.(string) == "independent" {
		filesetConfig.INodeSpace = "new"

		if inodeLimitSpecified {
			filesetConfig.MaxNumInodes = inodeLimit.(string)
		}
	}

	fileset := Fileset{Config: filesetConfig}
	createFilesetURL := utils.FormatURL(s.endpoint, "scalemgmt/v1/filesets")
	createFilesetResponse := CreateFilesetResponse{}
	response, err := s.doHTTP(createFilesetURL, "POST", createFilesetResponse, fileset)
	if err != nil {
		s.logger.Printf("error in remote call %v", err)
		return err
	}
	createFilesetResponse = response.(CreateFilesetResponse)
	//TODO check the response message content and code
	if createFilesetResponse.Status.Code != "0" {
		return fmt.Errorf("error creating fileset %v", createFilesetResponse)
	}
	return nil
}

func (s *spectrum_rest) DeleteFileset(filesystemName string, filesetName string) error {
	deleteFilesetURL := utils.FormatURL(s.endpoint, fmt.Sprintf("scalemgmt/v1/filesets/%s/filesystemName=%s&qosClass=other", filesetName, filesystemName))
	deleteFilesetResponse := DeleteFilesetResponse{}
	response, err := s.doHTTP(deleteFilesetURL, "DELETE", deleteFilesetResponse, nil)
	if err != nil {
		s.logger.Printf("Error in delete remote call")
		return err
	}

	deleteFilesetResponse = response.(DeleteFilesetResponse)
	if deleteFilesetResponse.Status.Code != "0" {
		return fmt.Errorf("error deleting fileset %v", deleteFilesetResponse)
	}

	return nil
}

func (s *spectrum_rest) LinkFileset(filesystemName string, filesetName string) error {
	filesetConfig := FilesetConfig{}
	filesetConfig.Comment = "fileset for container volume"
	filesetConfig.FilesetName = filesetName
	filesetConfig.FilesystemName = filesystemName
	fsMountpoint, err := s.GetFilesystemMountpoint(filesystemName)
	if err != nil {
		s.logger.Printf("error in linking fileset")
	}
	filesetConfig.Path = path.Join(fsMountpoint, filesetName)
	fileset := Fileset{Config: filesetConfig}
	linkFilesetURL := utils.FormatURL(s.endpoint, fmt.Sprintf("scalemgmt/v1/filesets/%s", filesetName))
	linkFilesetResponse := CreateFilesetResponse{}
	response, err := s.doHTTP(linkFilesetURL, "PUT", linkFilesetResponse, fileset)
	if err != nil {
		s.logger.Printf("error in remote call %v", err)
		return err
	}

	linkFilesetResponse = response.(CreateFilesetResponse)
	if linkFilesetResponse.Status.Code != "0" {
		return fmt.Errorf("error linking fileset %v", linkFilesetResponse)
	}
	return nil
}

func (s *spectrum_rest) UnlinkFileset(filesystemName string, filesetName string) error {
	filesetConfig := FilesetConfig{}
	filesetConfig.Comment = "fileset for container volume"
	filesetConfig.FilesetName = filesetName
	filesetConfig.FilesystemName = filesystemName
	filesetConfig.Path = ""
	fileset := Fileset{Config: filesetConfig}
	linkFilesetURL := utils.FormatURL(s.endpoint, fmt.Sprintf("scalemgmt/v1/filesets/%s", filesetName))
	linkFilesetResponse := CreateFilesetResponse{}
	response, err := s.doHTTP(linkFilesetURL, "PUT", linkFilesetResponse, fileset)
	if err != nil {
		s.logger.Printf("error in remote call %v", err)
		return err
	}

	linkFilesetResponse = response.(CreateFilesetResponse)
	if linkFilesetResponse.Status.Code != "0" {
		return fmt.Errorf("error unlinking fileset %v", linkFilesetResponse)
	}
	return nil
}

func (s *spectrum_rest) ListFilesets(filesystemName string) ([]resources.VolumeMetadata, error) {
	listFilesetURL := utils.FormatURL(s.endpoint, "scalemgmt/v1/filesets")
	listFilesetResponse := GetFilesetResponse{}
	lfsResponse, err := s.doHTTP(listFilesetURL, "GET", listFilesetResponse, nil)
	if err != nil {
		s.logger.Printf("error in processing remote call %v", err)
		return nil, err
	}

	listFilesetResponse = lfsResponse.(GetFilesetResponse)
	responseSize := len(listFilesetResponse.Filesets)
	response := make([]resources.VolumeMetadata, responseSize)

	for i := 0; i < responseSize; i++ {
		name := listFilesetResponse.Filesets[i].Config.FilesetName
		mountpoint := listFilesetResponse.Filesets[i].Config.Path
		response[i] = resources.VolumeMetadata{Name: name, Mountpoint: mountpoint}
	}
	return response, nil
}

func (s *spectrum_rest) ListFileset(filesystemName string, filesetName string) (resources.VolumeMetadata, error) {
	getFilesetURL := utils.FormatURL(s.endpoint, fmt.Sprintf("scalemgmt/v1/filesets/%s?filesystemname=%s", filesetName, filesystemName))
	getFilesetResponse := GetFilesetResponse{}
	gfsResponse, err := s.doHTTP(getFilesetURL, "GET", getFilesetResponse, nil)
	if err != nil {
		s.logger.Printf("error in processing remote call %v", err)
		return resources.VolumeMetadata{}, err
	}

	getFilesetResponse = gfsResponse.(GetFilesetResponse)
	name := getFilesetResponse.Filesets[0].Config.FilesetName
	mountpoint := getFilesetResponse.Filesets[0].Config.Path

	return resources.VolumeMetadata{Name: name, Mountpoint: mountpoint}, nil
}

func (s *spectrum_rest) IsFilesetLinked(filesystemName string, filesetName string) (bool, error) {
	fileset, err := s.ListFileset(filesystemName, filesetName)
	if err != nil {
		s.logger.Printf("error retrieving fileset data")
		return false, err
	}

	if fileset.Mountpoint == "" {
		return false, nil
	}
	return true, nil
}

//TODO modify quota from string to Capacity (see kubernetes)
func (s *spectrum_rest) ListFilesetQuota(filesystemName string, filesetName string) (string, error) {
	listQuotaURL := utils.FormatURL(s.endpoint, fmt.Sprintf("scalemgmt/v1/quotas?quotaType=fileset&filesystemName=%s&filesetName=%s", filesystemName, filesetName))
	listQuotaResponse := GetQuotaResponse{}
	gqResponse, err := s.doHTTP(listQuotaURL, "GET", listQuotaResponse, nil)
	if err != nil {
		s.logger.Printf("error in processing remote call %v", err)
		return "", err
	}

	listQuotaResponse = gqResponse.(GetQuotaResponse)
	//TODO check which quota in quotas[] and which attribute
	return listQuotaResponse.Quotas[0].BlockQuota, nil
}

func (s *spectrum_rest) SetFilesetQuota(filesystemName string, filesetName string, quota string) error {
	setQuotaURL := utils.FormatURL(s.endpoint, "scalemgmt/v1/quotas")
	quotaRequest := SetQuotaRequest{}
	quotaRequest.FilesetName = filesetName
	quotaRequest.FilesystemName = filesystemName
	quotaRequest.BlockHardLimit = quota
	quotaRequest.BlockSoftLimit = quota
	quotaRequest.OperationType = "setQuota"
	quotaRequest.QuotaType = "fileset"
	setQuotaResponse := SetQuotaResponse{}
	sqResponse, err := s.doHTTP(setQuotaURL, "POST", setQuotaResponse, quotaRequest)
	if err != nil {
		s.logger.Printf("error setting quota for fileset %v", err)
		return err
	}
	setQuotaResponse = sqResponse.(SetQuotaResponse)
	if setQuotaResponse.Status.Code != "0" {
		return fmt.Errorf("error unlinking fileset %v", setQuotaResponse)
	}
	return nil
}

func (s *spectrum_rest) doHTTP(endpoint string, method string, responseObject interface{}, param interface{}) (interface{}, error) {
	response, err := utils.HttpExecute(s.httpClient, s.logger, method, endpoint, param)
	if err != nil {
		s.logger.Printf("Error in %s: %s remote call %#v", method, endpoint, err)
		return nil, fmt.Errorf("Error in get filesystem remote call")
	}

	if response.StatusCode != http.StatusOK {
		s.logger.Printf("Error in get filesystem remote call %#v\n", response)
		return nil, utils.ExtractErrorResponse(response)
	}
	err = utils.UnmarshalResponse(response, &responseObject)
	if err != nil {
		s.logger.Printf("Error in unmarshalling response for get remote call %#v for response %#v", err, response)
		return nil, fmt.Errorf("Error in unmarshalling response for get remote call")
	}

	return responseObject, nil
}
