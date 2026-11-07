// Package erplink links an approved org to its Lebane ERP identity (lane D, R-ERP-1…3): the
// identity index (doc type + number → ERP org/client) answers candidates in milliseconds, a
// link needs an extra ERP credentials check through the Directory port, and the import job
// copies ERP projects as project accounts and ERP buyers as payers (snapshot; the app never
// calls the ERP live). The periodic erp_index_refresh job (re)loads erp_identity_index from
// the Directory: the JSON fixtures today, the ERP dump or CDC read model later, behind the
// same port. It owns erp_identity_index and erp_links.
package erplink
