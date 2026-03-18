use serde::{Serialize, Deserialize};
use uuid::{Uuid, Timestamp, NoContext};

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct CreateShortUrlRequest {
    original_url: String,
}


pub struct ShortUrl {   
    original_url: String,
    short_url: String,
    created_at: String,
    is_active: bool,
    updated_at: String,
    id : uuid::Uuid, 
}

pub fn create_short_url(original_url: &str, short_url: &str) -> ShortUrl {
    let ts = Timestamp::now(NoContext);
    let short_url = ShortUrl {
        original_url: original_url.to_string(),
        short_url: short_url.to_string(),
        created_at: chrono::Utc::now().to_rfc3339(),
        is_active: true,
        updated_at: chrono::Utc::now().to_rfc3339(),
        id: Uuid::new_v7(ts),
    };
    return short_url ;
}



mod tests {
    

    use super::*;

    #[test]
    fn test_parse_create_short_url_request_valid_payload() {
        let json = r#"{"original_url": "https://www.example.com"}"#;
        let result :  CreateShortUrlRequest = match serde_json::from_str(json) {
            Ok(res) => res,
            Err(e) => panic!("Failed to parse JSON: {}", e),
         };
        assert_eq!(result.original_url, "https://www.example.com");

        }


    #[test]
    fn test_parse_create_short_url_request_invalid_payload() {
        let json = r#"{"original_url": 123}"#;
        let result: Result<CreateShortUrlRequest, serde_json::Error>  =  serde_json::from_str(json);
        assert!(result.is_err());
        
    }
}